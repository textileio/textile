package gateway

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/textileio/go-threads/core/thread"

	"github.com/gin-contrib/location"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	logging "github.com/ipfs/go-log"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/rs/cors"
	gincors "github.com/rs/cors/wrapper/gin"
	threadsclient "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/broadcast"
	tutil "github.com/textileio/go-threads/util"
	bucketsclient "github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/collections"
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
	sync.Mutex

	server        *http.Server
	addr          ma.Multiaddr
	url           string
	bucketsDomain string

	collections *collections.Collections
	apiSession  string
	threads     *threadsclient.Client
	buckets     *bucketsclient.Client

	ipfs iface.CoreAPI

	sessionBus *broadcast.Broadcaster
}

// Config defines the gateway configuration.
type Config struct {
	Addr          ma.Multiaddr
	URL           string
	BucketsDomain string
	APIAddr       ma.Multiaddr
	APISession    string
	Collections   *collections.Collections
	IPFSClient    iface.CoreAPI
	SessionBus    *broadcast.Broadcaster
	Debug         bool
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
		addr:          conf.Addr,
		url:           conf.URL,
		bucketsDomain: conf.BucketsDomain,
		collections:   conf.Collections,
		apiSession:    conf.APISession,
		threads:       tc,
		buckets:       bc,
		ipfs:          conf.IPFSClient,
		sessionBus:    conf.SessionBus,
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

	router.GET("/thread/:thread/:collection", g.namespaceHandler)
	router.GET("/thread/:thread/:collection/:id", g.namespaceHandler)
	router.GET("/thread/:thread/:collection/:id/*path", g.namespaceHandler)

	router.GET("/ipfs/:root", g.namespaceHandler)
	router.GET("/ipfs/:root/*path", g.namespaceHandler)
	router.GET("/ipns/:root", g.namespaceHandler)
	router.GET("/ipns/:root/*path", g.namespaceHandler)
	router.GET("/p2p/:root", g.namespaceHandler)
	router.GET("/p2p/:root/*path", g.namespaceHandler)
	router.GET("/ipld/:root", g.namespaceHandler)
	router.GET("/ipld/:root/*path", g.namespaceHandler)

	router.GET("/dashboard/:username", g.dashboardHandler)
	router.GET("/confirm/:secret", g.confirmEmail)
	router.GET("/consent/:invite", g.consentInvite)

	router.NoRoute(g.routeSubdomains)

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

// namespaceHandler redirects IPFS namespaces to their subdomain equivalents.
func (g *Gateway) namespaceHandler(c *gin.Context) {
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
	if err := g.sessionBus.Send(c.Param("secret")); err != nil {
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
			if err := g.collections.Accounts.AddMember(ctx, invite.Org, collections.Member{
				Key:      dev.Key,
				Username: dev.Username,
				Role:     collections.OrgMember,
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

// routeSubdomains handles the request with the matching subdomain renderer.
func (g *Gateway) routeSubdomains(c *gin.Context) {
	host := strings.SplitN(c.Request.Host, ":", 2)[0]
	parts := strings.Split(host, ".")
	key := parts[0]

	// Render buckets if the domain matches
	if strings.HasSuffix(host, g.bucketsDomain) {
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
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()
	switch ns {
	case "ipfs":
		g.renderIPFSPath(c, ctx, path.New(key+c.Request.URL.Path))
	case "ipns":
		pth, err := g.ipfs.Name().Resolve(ctx, key)
		if err != nil {
			renderError(c, http.StatusNotFound, err)
			return
		}
		g.renderIPFSPath(c, ctx, path.New(pth.String()+c.Request.URL.Path))
	case "p2p":
		pid, err := peer.Decode(key)
		if err != nil {
			renderError(c, http.StatusBadRequest, err)
			return
		}
		info, err := g.ipfs.Dht().FindPeer(ctx, pid)
		if err != nil {
			renderError(c, http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusOK, info)
	case "ipld":
		node, err := g.ipfs.Object().Get(ctx, path.New(key+c.Request.URL.Path))
		if err != nil {
			renderError(c, http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusOK, node)
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
				g.collectionHandler(c, threadID, parts[1])
			} else {
				render404(c)
			}
		case 3:
			g.instanceHandler(c, threadID, parts[1], parts[2], "")
		case 4:
			g.instanceHandler(c, threadID, parts[1], parts[2], parts[3])
		default:
			render404(c)
		}
	default:
		render404(c)
	}
}
