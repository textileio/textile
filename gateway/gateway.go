package gateway

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/location"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	logger "github.com/ipfs/go-log"
	logging "github.com/ipfs/go-log"
	assets "github.com/jessevdk/go-assets"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/rs/cors"
	gincors "github.com/rs/cors/wrapper/gin"
	threadsclient "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/broadcast"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/api/buckets"
	bucketsclient "github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/collections"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
)

var log = logger.Logger("gateway")

const handlerTimeout = time.Second * 10

func init() {
	gin.SetMode(gin.ReleaseMode)
}

// fileSystem extends the binary asset file system with Exists,
// enabling its use with the static middleware.
type fileSystem struct {
	*assets.FileSystem
}

// Exists returns whether or not the path exists in the binary assets.
func (f *fileSystem) Exists(prefix, path string) bool {
	pth := strings.TrimPrefix(path, prefix)
	if pth == "/" {
		return false
	}
	_, ok := f.Files[pth]
	return ok
}

// Gateway provides HTTP-based access to Textile.
type Gateway struct {
	addr         ma.Multiaddr
	bucketDomain string
	server       *http.Server
	collections  *collections.Collections
	buckets      *bucketsclient.Client
	threads      *threadsclient.Client
	session      string
	sessionBus   *broadcast.Broadcaster
}

// NewGateway returns a new gateway.
func NewGateway(addr, apiAddr ma.Multiaddr, session, bucketDomain string, collections *collections.Collections, sessionBus *broadcast.Broadcaster, debug bool) (*Gateway, error) {
	if debug {
		if err := tutil.SetLogLevels(map[string]logging.LogLevel{
			"gateway": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	apiTarget, err := tutil.TCPAddrFromMultiAddr(apiAddr)
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
		addr:         addr,
		bucketDomain: bucketDomain,
		collections:  collections,
		threads:      tc,
		buckets:      bc,
		session:      session,
		sessionBus:   sessionBus,
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
	router.Use(serveBucket(&bucketFileSystem{
		client:  g.buckets,
		session: g.session,
		timeout: handlerTimeout,
		host:    g.bucketDomain,
	}))
	router.Use(gincors.New(cors.Options{}))

	router.GET("/health", func(c *gin.Context) {
		c.Writer.WriteHeader(http.StatusNoContent)
	})

	router.GET("", g.renderBucketHandler)
	router.GET("/thread/:thread/:collection", g.collectionHandler)
	router.GET("/thread/:thread/:collection/:id", g.instanceHandler)
	router.GET("/thread/:thread/:collection/:id/*path", g.instanceHandler)
	router.GET("/dashboard/:name", g.dashboardHandler)
	router.GET("/confirm/:secret", g.confirmEmail)
	router.GET("/consent/:invite", g.consentInvite)

	router.NoRoute(func(c *gin.Context) {
		render404(c)
	})

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

// renderBucketHandler renders a bucket as a website.
func (g *Gateway) renderBucketHandler(c *gin.Context) {
	threadID, slug, err := bucketFromHost(c.Request.Host, g.bucketDomain)
	if err != nil {
		renderError(c, http.StatusBadRequest, err)
		return
	}

	ctx, cancel := context.WithTimeout(common.NewSessionContext(context.Background(), g.session), handlerTimeout)
	defer cancel()
	ctx = common.NewThreadIDContext(ctx, threadID)
	token := thread.Token(c.Query("token"))
	if token.Defined() {
		ctx = thread.NewTokenContext(ctx, token)
	}

	query := db.Where("slug").Eq(slug)
	res, err := g.threads.Find(ctx, threadID, "buckets", query, &buckets.Bucket{}, db.WithTxnToken(token))
	if err != nil {
		renderError(c, http.StatusInternalServerError, err)
		return
	}
	bucks := res.([]*buckets.Bucket)
	if len(bucks) == 0 {
		render404(c)
		return
	}
	buck := bucks[0]

	rep, err := g.buckets.ListPath(ctx, buck.Name)
	if err != nil {
		renderError(c, http.StatusInternalServerError, err)
		return
	}
	for _, item := range rep.Item.Items {
		if item.Name == "index.html" {
			c.Writer.WriteHeader(http.StatusOK)
			c.Writer.Header().Set("Content-Type", "text/html")
			pth := strings.Replace(item.Path, rep.Root.Path, rep.Root.Name, 1)
			if err := g.buckets.PullPath(ctx, pth, c.Writer); err != nil {
				renderError(c, http.StatusInternalServerError, err)
			}
			return
		}
	}
	render404(c)
}

type link struct {
	ID    string
	Name  string
	Path  string
	Size  string
	Links string
}

// dashboardHandler renders a dev or org dashboard.
func (g *Gateway) dashboardHandler(c *gin.Context) {
	render404(c)
}

// collectionHandler renders all instances in a collection.
func (g *Gateway) collectionHandler(c *gin.Context) {
	collection := c.Param("collection")

	ctx, cancel := context.WithTimeout(common.NewSessionContext(context.Background(), g.session), handlerTimeout)
	defer cancel()
	threadID, err := thread.Decode(c.Param("thread"))
	if err != nil {
		renderError(c, http.StatusBadRequest, fmt.Errorf("thread is not valid"))
		return
	}
	ctx = common.NewThreadIDContext(ctx, threadID)
	token := thread.Token(c.Query("token"))
	if token.Defined() {
		ctx = thread.NewTokenContext(ctx, token)
	}

	switch collection {
	case "buckets":
		rep, err := g.buckets.ListPath(ctx, "")
		if err != nil {
			renderError(c, http.StatusBadRequest, err)
			return
		}
		links := make([]link, len(rep.Item.Items))
		for i, item := range rep.Item.Items {
			var pth string
			if rep.Root != nil {
				pth = strings.Replace(item.Path, rep.Root.Path, rep.Root.ID, 1)
			} else {
				pth = item.ID
			}
			links[i] = link{
				ID:    item.ID,
				Name:  item.Name,
				Path:  path.Join("thread", threadID.String(), "buckets", pth),
				Size:  byteCountDecimal(item.Size),
				Links: strconv.Itoa(len(item.Items)),
			}
		}
		c.HTML(http.StatusOK, "/public/html/buckets.gohtml", gin.H{
			"Path":  rep.Item.Path,
			"Root":  "",
			"Back":  "",
			"Links": links,
		})
	default:
		var dummy interface{}
		res, err := g.threads.Find(ctx, threadID, collection, &db.Query{}, dummy, db.WithTxnToken(token))
		if err != nil {
			renderError(c, http.StatusInternalServerError, err)
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

// instanceHandler renders an instance in a collection.
// If the collection is bucket, the built-in buckets UI in rendered instead.
// This can be overridden with the query param json=true.
func (g *Gateway) instanceHandler(c *gin.Context) {
	collection := c.Param("collection")
	json := c.Param("json") == "true"
	if (collection != "buckets" || json) && c.Param("path") != "" {
		render404(c)
		return
	}

	ctx, cancel := context.WithTimeout(common.NewSessionContext(context.Background(), g.session), handlerTimeout)
	defer cancel()
	threadID, err := thread.Decode(c.Param("thread"))
	if err != nil {
		renderError(c, http.StatusBadRequest, fmt.Errorf("thread is not valid"))
		return
	}
	ctx = common.NewThreadIDContext(ctx, threadID)
	token := thread.Token(c.Query("token"))
	if token.Defined() {
		ctx = thread.NewTokenContext(ctx, token)
	}

	if collection == "buckets" && !json {
		var buck buckets.Bucket
		if err := g.threads.FindByID(ctx, threadID, collection, c.Param("id"), &buck, db.WithTxnToken(token)); err != nil {
			render404(c)
			return
		}
		bpth := path.Join(buck.Name, c.Param("path"))
		rep, err := g.buckets.ListPath(ctx, bpth)
		if err != nil {
			render404(c)
			return
		}
		if !rep.Item.IsDir {
			if err := g.buckets.PullPath(ctx, bpth, c.Writer); err != nil {
				renderError(c, http.StatusInternalServerError, err)
			}
		} else {
			base := path.Join("thread", threadID.String(), collection)
			links := make([]link, len(rep.Item.Items))
			for i, item := range rep.Item.Items {
				var pth string
				if rep.Root != nil {
					pth = strings.Replace(item.Path, rep.Root.Path, rep.Root.ID, 1)
				} else {
					pth = item.ID
				}
				links[i] = link{
					ID:    item.ID,
					Name:  item.Name,
					Path:  path.Join(base, pth),
					Size:  byteCountDecimal(item.Size),
					Links: strconv.Itoa(len(item.Items)),
				}
			}
			var root, back string
			if rep.Root != nil {
				root = strings.Replace(rep.Item.Path, rep.Root.Path, rep.Root.Name, 1)
				back = path.Dir(path.Join(base, strings.Replace(rep.Item.Path, rep.Root.Path, rep.Root.ID, 1)))
			} else {
				back = path.Join(base, buck.ID)
			}
			c.HTML(http.StatusOK, "/public/html/buckets.gohtml", gin.H{
				"Path":  rep.Item.Path,
				"Root":  root,
				"Back":  back,
				"Links": links,
			})
		}
	} else {
		var res interface{}
		if err := g.threads.FindByID(ctx, threadID, collection, c.Param("id"), &res, db.WithTxnToken(token)); err != nil {
			render404(c)
			return
		}
		c.JSON(http.StatusOK, res)
	}
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

// formatError formats a go error for browser display.
func formatError(err error) string {
	words := strings.SplitN(err.Error(), " ", 2)
	words[0] = strings.Title(words[0])
	return strings.Join(words, " ") + "."
}

// byteCountDecimal formats bytes
func byteCountDecimal(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}
