package gateway

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-contrib/location"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	iface "github.com/ipfs/interface-go-ipfs-core"
	isd "github.com/jbenet/go-is-domain"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	mbase "github.com/multiformats/go-multibase"
	"github.com/rs/cors"
	gincors "github.com/rs/cors/wrapper/gin"
	threadsclient "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/broadcast"
	"github.com/textileio/go-threads/core/thread"
	tutil "github.com/textileio/go-threads/util"
	bucketsclient "github.com/textileio/textile/v2/api/bucketsd/client"
	"github.com/textileio/textile/v2/api/common"
	mdb "github.com/textileio/textile/v2/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
)

var log = logging.Logger("gateway")

const handlerTimeout = time.Minute

func init() {
	gin.SetMode(gin.ReleaseMode)
}

// link is used for Unixfs directory templates.
type link struct {
	Name  string
	Path  string
	Size  string
	Links string
}

// Gateway provides HTTP-based access to Textile.
type Gateway struct {
	server        *http.Server
	addr          ma.Multiaddr
	url           string
	subdomains    bool
	bucketsDomain string

	collections *mdb.Collections
	apiSession  string
	threads     *threadsclient.Client
	buckets     *bucketsclient.Client
	hub         bool

	ipfs iface.CoreAPI

	emailSessionBus *broadcast.Broadcaster
}

// Config defines the gateway configuration.
type Config struct {
	Addr            ma.Multiaddr
	URL             string
	Subdomains      bool
	BucketsDomain   string
	APIAddr         ma.Multiaddr
	APISession      string
	Collections     *mdb.Collections
	IPFSClient      iface.CoreAPI
	EmailSessionBus *broadcast.Broadcaster
	Hub             bool
	Debug           bool
}

// NewGateway returns a new gateway.
func NewGateway(conf Config) (*Gateway, error) {
	if conf.Debug {
		if err := tutil.SetLogLevels(map[string]logging.LogLevel{
			"gateway": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	apiTarget, err := tutil.TCPAddrFromMultiAddr(conf.APIAddr)
	if err != nil {
		return nil, err
	}
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithPerRPCCredentials(common.Credentials{}),
	}
	tc, err := threadsclient.NewClient(apiTarget, opts...)
	if err != nil {
		return nil, err
	}
	bc, err := bucketsclient.NewClient(apiTarget, opts...)
	if err != nil {
		return nil, err
	}
	return &Gateway{
		addr:            conf.Addr,
		url:             conf.URL,
		subdomains:      conf.Subdomains,
		bucketsDomain:   conf.BucketsDomain,
		collections:     conf.Collections,
		apiSession:      conf.APISession,
		threads:         tc,
		buckets:         bc,
		hub:             conf.Hub,
		ipfs:            conf.IPFSClient,
		emailSessionBus: conf.EmailSessionBus,
	}, nil
}

// Start the gateway.
func (g *Gateway) Start() {
	addr, err := tutil.TCPAddrFromMultiAddr(g.addr)
	if err != nil {
		log.Fatal(err)
	}
	router := gin.Default()

	temp, err := loadTemplate()
	if err != nil {
		log.Fatal(err)
	}
	router.SetHTMLTemplate(temp)

	router.Use(location.Default())
	router.Use(static.Serve("", &fileSystem{Assets}))
	router.Use(serveBucket(&bucketFS{
		client:  g.buckets,
		keys:    g.collections.IPNSKeys,
		session: g.apiSession,
		host:    g.bucketsDomain,
	}))
	router.Use(gincors.New(cors.Options{}))

	router.GET("/health", func(c *gin.Context) {
		c.Writer.WriteHeader(http.StatusNoContent)
	})

	router.GET("/thread/:thread/:collection", g.subdomainOptionHandler, g.collectionHandler)
	router.GET("/thread/:thread/:collection/:id", g.subdomainOptionHandler, g.instanceHandler)
	router.GET("/thread/:thread/:collection/:id/*path", g.subdomainOptionHandler, g.instanceHandler)

	router.GET("/ipfs/:root", g.subdomainOptionHandler, g.ipfsHandler)
	router.GET("/ipfs/:root/*path", g.subdomainOptionHandler, g.ipfsHandler)
	router.GET("/ipns/:key", g.subdomainOptionHandler, g.ipnsHandler)
	router.GET("/ipns/:key/*path", g.subdomainOptionHandler, g.ipnsHandler)
	router.GET("/p2p/:key", g.subdomainOptionHandler, g.p2pHandler)
	router.GET("/ipld/:root", g.subdomainOptionHandler, g.ipldHandler)
	router.GET("/ipld/:root/*path", g.subdomainOptionHandler, g.ipldHandler)

	if g.hub {
		router.GET("/dashboard/:username", g.dashboardHandler)
		router.GET("/confirm/:secret", g.confirmEmail)
		router.GET("/consent/:invite", g.consentInvite)
	}

	router.NoRoute(g.subdomainHandler)

	g.server = &http.Server{
		Addr:    addr,
		Handler: router,
	}
	go func() {
		if err := g.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("gateway error: %s", err)
		}
		log.Info("gateway was shutdown")
	}()
	log.Infof("gateway listening at %s", g.server.Addr)
}

// loadTemplate loads HTML templates.
func loadTemplate() (*template.Template, error) {
	t := template.New("")
	for name, file := range Assets.Files {
		if file.IsDir() || !strings.HasSuffix(name, ".gohtml") {
			continue
		}
		h, err := ioutil.ReadAll(file)
		if err != nil {
			return nil, err
		}
		t, err = t.New(name).Parse(string(h))
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}

// Addr returns the gateway's address.
func (g *Gateway) Addr() string {
	return g.server.Addr
}

// Stop the gateway.
func (g *Gateway) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := g.server.Shutdown(ctx); err != nil {
		return err
	}
	if err := g.threads.Close(); err != nil {
		return err
	}
	if err := g.buckets.Close(); err != nil {
		return err
	}
	return nil
}

// subdomainOptionHandler redirects valid namespaces to subdomains if the option is enabled.
func (g *Gateway) subdomainOptionHandler(c *gin.Context) {
	if !g.subdomains {
		return
	}
	loc, ok := g.toSubdomainURL(c.Request)
	if !ok {
		render404(c)
		return
	}

	// See security note https://github.com/ipfs/go-ipfs/blob/dbfa7bf2b216bad9bec1ff66b1f3814f4faac31e/core/corehttp/hostname.go#L105
	c.Request.Header.Set("Clear-Site-Data", "\"cookies\", \"storage\"")

	c.Redirect(http.StatusPermanentRedirect, loc)
}

// dashboardHandler renders a dev or org dashboard.
func (g *Gateway) dashboardHandler(c *gin.Context) {
	render404(c)
}

// confirmEmail verifies an emailed secret.
func (g *Gateway) confirmEmail(c *gin.Context) {
	if err := g.emailSessionBus.Send(c.Param("secret")); err != nil {
		renderError(c, http.StatusInternalServerError, err)
		return
	}
	c.HTML(http.StatusOK, "/public/html/confirm.gohtml", nil)
}

// consentInvite marks an invite as accepted.
// If the associated email belongs to an existing user, they will be added to the org.
func (g *Gateway) consentInvite(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()
	invite, err := g.collections.Invites.Get(ctx, c.Param("invite"))
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			render404(c)
		} else {
			renderError(c, http.StatusInternalServerError, err)
		}
		return
	}
	if !invite.Accepted {
		if time.Now().After(invite.ExpiresAt) {
			if err := g.collections.Invites.Delete(ctx, invite.Token); err != nil {
				renderError(c, http.StatusInternalServerError, err)
			} else {
				renderError(c, http.StatusPreconditionFailed, fmt.Errorf("this invitation has expired"))
			}
			return
		}
		dev, err := g.collections.Accounts.GetByUsernameOrEmail(ctx, invite.EmailTo)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				if err := g.collections.Invites.Accept(ctx, invite.Token); err != nil {
					renderError(c, http.StatusInternalServerError, err)
				}
			} else {
				renderError(c, http.StatusInternalServerError, err)
				return
			}
		}
		if dev != nil {
			if err := g.collections.Accounts.AddMember(ctx, invite.Org, mdb.Member{
				Key:      dev.Key,
				Username: dev.Username,
				Role:     mdb.OrgMember,
			}); err != nil {
				if err == mongo.ErrNoDocuments {
					if err := g.collections.Invites.Delete(ctx, invite.Token); err != nil {
						renderError(c, http.StatusInternalServerError, err)

					} else {
						renderError(c, http.StatusNotFound, fmt.Errorf("org not found"))
					}
				} else {
					renderError(c, http.StatusInternalServerError, err)
				}
				return
			}
			if err = g.collections.Invites.Delete(ctx, invite.Token); err != nil {
				renderError(c, http.StatusInternalServerError, err)
				return
			}
		}
	}
	c.HTML(http.StatusOK, "/public/html/consent.gohtml", gin.H{
		"Org":   invite.Org,
		"Email": invite.EmailTo,
	})
}

// render404 renders the 404 template.
func render404(c *gin.Context) {
	c.HTML(http.StatusNotFound, "/public/html/404.gohtml", nil)
}

// renderError renders the error template.
func renderError(c *gin.Context, code int, err error) {
	c.HTML(code, "/public/html/error.gohtml", gin.H{
		"Code":  code,
		"Error": formatError(err),
	})
}

// formatError formats a go error for browser display.
func formatError(err error) string {
	words := strings.SplitN(err.Error(), " ", 2)
	words[0] = strings.Title(words[0])
	return strings.Join(words, " ") + "."
}

// subdomainHandler handles requests by parsing the request subdomain.
func (g *Gateway) subdomainHandler(c *gin.Context) {
	c.Status(200)

	parts := strings.Split(c.Request.Host, ".")
	key := parts[0]

	// Render buckets if the domain matches
	if g.bucketsDomain != "" && strings.HasSuffix(c.Request.Host, g.bucketsDomain) {
		g.renderWWWBucket(c, key)
		return
	}

	if len(parts) < 3 {
		render404(c)
		return
	}
	ns := parts[1]
	if !isSubdomainNamespace(ns) {
		render404(c)
		return
	}
	switch ns {
	case "ipfs":
		g.renderIPFSPath(c, "ipfs/"+key, "/ipfs/"+key+c.Request.URL.Path)
	case "ipns":
		g.renderIPNSKey(c, key, c.Request.URL.Path)
	case "p2p":
		g.renderP2PKey(c, key)
	case "ipld":
		g.renderIPLDPath(c, key+c.Request.URL.Path)
	case "thread":
		threadID, err := thread.Decode(key)
		if err != nil {
			renderError(c, http.StatusBadRequest, fmt.Errorf("invalid thread ID"))
			return
		}
		parts := strings.SplitN(strings.TrimSuffix(c.Request.URL.Path, "/"), "/", 4)
		switch len(parts) {
		case 1:
			// @todo: Render something at the thread root
			render404(c)
		case 2:
			if parts[1] != "" {
				g.renderCollection(c, threadID, parts[1])
			} else {
				render404(c)
			}
		case 3:
			g.renderInstance(c, threadID, parts[1], parts[2], "")
		case 4:
			g.renderInstance(c, threadID, parts[1], parts[2], parts[3])
		default:
			render404(c)
		}
	default:
		render404(c)
	}
}

// Modified from https://github.com/ipfs/go-ipfs/blob/dbfa7bf2b216bad9bec1ff66b1f3814f4faac31e/core/corehttp/hostname.go#L251
func isSubdomainNamespace(ns string) bool {
	switch ns {
	case "ipfs", "ipns", "p2p", "ipld", "thread":
		return true
	default:
		return false
	}
}

// Copied from https://github.com/ipfs/go-ipfs/blob/dbfa7bf2b216bad9bec1ff66b1f3814f4faac31e/core/corehttp/hostname.go#L260
func isPeerIDNamespace(ns string) bool {
	switch ns {
	case "ipns", "p2p":
		return true
	default:
		return false
	}
}

// Converts a hostname/path to a subdomain-based URL, if applicable.
// Modified from https://github.com/ipfs/go-ipfs/blob/dbfa7bf2b216bad9bec1ff66b1f3814f4faac31e/core/corehttp/hostname.go#L270
func (g *Gateway) toSubdomainURL(r *http.Request) (redirURL string, ok bool) {
	var ns, rootID, rest string

	query := r.URL.RawQuery
	parts := strings.SplitN(r.URL.Path, "/", 4)
	safeRedirectURL := func(in string) (out string, ok bool) {
		safeURI, err := url.ParseRequestURI(in)
		if err != nil {
			return "", false
		}
		return safeURI.String(), true
	}

	switch len(parts) {
	case 4:
		rest = parts[3]
		fallthrough
	case 3:
		ns = parts[1]
		rootID = parts[2]
	default:
		return "", false
	}

	if !isSubdomainNamespace(ns) {
		return "", false
	}

	// add prefix if query is present
	if query != "" {
		query = "?" + query
	}

	// Normalize problematic PeerIDs (eg. ed25519+identity) to CID representation
	if isPeerIDNamespace(ns) && !isd.IsDomain(rootID) {
		peerID, err := peer.Decode(rootID)
		// Note: PeerID CIDv1 with protobuf multicodec will fail, but we fix it
		// in the next block
		if err == nil {
			rootID = peer.ToCid(peerID).String()
		}
	}

	// If rootID is a CID, ensure it uses DNS-friendly text representation
	if rootCid, err := cid.Decode(rootID); err == nil {
		multicodec := rootCid.Type()

		// PeerIDs represented as CIDv1 are expected to have libp2p-key
		// multicodec (https://github.com/libp2p/specs/pull/209).
		// We ease the transition by fixing multicodec on the fly:
		// https://github.com/ipfs/go-ipfs/issues/5287#issuecomment-492163929
		if isPeerIDNamespace(ns) && multicodec != cid.Libp2pKey {
			multicodec = cid.Libp2pKey
		}

		// if object turns out to be a valid CID,
		// ensure text representation used in subdomain is CIDv1 in Base32
		// https://github.com/ipfs/in-web-browsers/issues/89
		rootID, err = cid.NewCidV1(multicodec, rootCid.Hash()).StringOfBase(mbase.Base32)
		if err != nil {
			// should not error, but if it does, its clealy not possible to
			// produce a subdomain URL
			return "", false
		}
	}

	urlparts := strings.Split(g.url, "://")
	if len(urlparts) < 2 {
		return "", false
	}
	scheme := urlparts[0]
	host := urlparts[1]
	return safeRedirectURL(fmt.Sprintf("%s://%s.%s.%s/%s%s", scheme, rootID, ns, host, rest, query))
}
