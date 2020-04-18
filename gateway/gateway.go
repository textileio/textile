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
	"github.com/textileio/go-threads/broadcast"
	tutil "github.com/textileio/go-threads/util"
	buckets "github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/api/common"
	hub "github.com/textileio/textile/api/hub/client"
	"github.com/textileio/textile/collections"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
)

const handlerTimeout = time.Second * 10

var log = logger.Logger("gateway")

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
	collections  *collections.Collections // @todo: Remove in favor of the client
	hub          *hub.Client
	buckets      *buckets.Client
	token        string
	sessionBus   *broadcast.Broadcaster
}

// NewGateway returns a new gateway.
func NewGateway(addr, apiAddr ma.Multiaddr, apiToken, bucketDomain string, collections *collections.Collections, sessionBus *broadcast.Broadcaster, debug bool) (*Gateway, error) {
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
	cc, err := hub.NewClient(apiTarget, opts...)
	if err != nil {
		return nil, err
	}
	bc, err := buckets.NewClient(apiTarget, opts...)
	if err != nil {
		return nil, err
	}
	return &Gateway{
		addr:         addr,
		bucketDomain: bucketDomain,
		collections:  collections,
		hub:          cc,
		buckets:      bc,
		token:        apiToken,
		sessionBus:   sessionBus,
	}, nil
}

// Start the gateway.
func (g *Gateway) Start() {
	addr, err := tutil.TCPAddrFromMultiAddr(g.addr)
	if err != nil {
		log.Fatal(err)
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.Use(location.Default())

	// @todo: Config based headers
	options := cors.Options{}
	router.Use(gincors.New(options))

	temp, err := loadTemplate()
	if err != nil {
		log.Fatal(err)
	}
	router.SetHTMLTemplate(temp)

	router.Use(static.Serve("", &fileSystem{Assets}))

	router.Use(serveBucket(&bucketFileSystem{
		client:  g.buckets,
		token:   g.token,
		timeout: handlerTimeout,
		host:    g.bucketDomain,
	}))

	router.GET("/health", func(c *gin.Context) {
		c.Writer.WriteHeader(http.StatusNoContent)
	})

	router.GET("", g.bucketHandler)

	router.GET("/dashboard/:project", g.dashHandler)
	router.GET("/dashboard/:project/*path", g.dashHandler)

	router.GET("/confirm/:secret", g.confirmEmail)
	router.GET("/consent/:invite", g.consentInvite)

	router.NoRoute(func(c *gin.Context) {
		g.render404(c)
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

// APIToken returns the gateway's internal API token.
func (g *Gateway) APIToken() string {
	return g.token
}

// Stop the gateway.
func (g *Gateway) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := g.server.Shutdown(ctx); err != nil {
		return err
	}
	if err := g.hub.Close(); err != nil {
		return err
	}
	if err := g.buckets.Close(); err != nil {
		return err
	}
	return nil
}

// bucketHandler renders a bucket as a website.
func (g *Gateway) bucketHandler(c *gin.Context) {
	log.Debugf("host: %s", c.Request.Host)

	buckName, err := bucketFromHost(c.Request.Host, g.bucketDomain)
	if err != nil {
		abort(c, http.StatusBadRequest, err)
		return
	}

	ctx, cancel := context.WithTimeout(common.NewSessionContext(context.Background(), g.token), handlerTimeout)
	defer cancel()
	rep, err := g.buckets.ListPath(ctx, buckName)
	if err != nil {
		abort(c, http.StatusInternalServerError, err)
		return
	}

	for _, item := range rep.Item.Items {
		if item.Name == "index.html" {
			c.Writer.WriteHeader(http.StatusOK)
			c.Writer.Header().Set("Content-Type", "text/html")
			pth := strings.Replace(item.Path, rep.Root.Path, rep.Root.Name, 1)
			if err := g.buckets.PullPath(ctx, pth, c.Writer); err != nil {
				abort(c, http.StatusInternalServerError, err)
			}
			return
		}
	}

	// No index was found, use the default (404 for now)
	g.render404(c)
}

type link struct {
	Name  string
	Path  string
	Size  string
	Links string
}

// dashHandler renders a project dashboard.
// Currently, this just shows bucket files and directories.
func (g *Gateway) dashHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(common.NewSessionContext(context.Background(), g.token), handlerTimeout)
	defer cancel()

	project := c.Param("project")
	rep, err := g.buckets.ListPath(ctx, c.Param("path"))
	if err != nil {
		abort(c, http.StatusNotFound, fmt.Errorf("project not found"))
		return
	}

	if !rep.Item.IsDir {
		if err := g.buckets.PullPath(ctx, c.Param("path"), c.Writer); err != nil {
			abort(c, http.StatusInternalServerError, err)
		}
	} else {
		projectPath := path.Join("dashboard", project)

		links := make([]link, len(rep.Item.Items))
		for i, item := range rep.Item.Items {
			var pth string
			if rep.Root != nil {
				pth = strings.Replace(item.Path, rep.Root.Path, rep.Root.Name, 1)
			} else {
				pth = item.Name
			}

			links[i] = link{
				Name:  item.Name,
				Path:  path.Join(projectPath, pth),
				Size:  byteCountDecimal(item.Size),
				Links: strconv.Itoa(len(item.Items)),
			}
		}

		var root, back string
		if rep.Root != nil {
			root = strings.Replace(rep.Item.Path, rep.Root.Path, rep.Root.Name, 1)
		} else {
			root = ""
		}
		if root == "" {
			back = projectPath
		} else {
			back = path.Dir(path.Join(projectPath, root))
		}
		c.HTML(http.StatusOK, "/public/html/buckets.gohtml", gin.H{
			"Path":  rep.Item.Path,
			"Root":  root,
			"Back":  back,
			"Links": links,
		})
	}
}

// confirmEmail verifies an emailed secret.
func (g *Gateway) confirmEmail(c *gin.Context) {
	if err := g.sessionBus.Send(c.Param("secret")); err != nil {
		g.renderError(c, http.StatusInternalServerError, err)
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
			g.render404(c)
		} else {
			g.renderError(c, http.StatusInternalServerError, err)
		}
		return
	}
	if !invite.Accepted {
		if time.Now().After(invite.ExpiresAt) {
			if err := g.collections.Invites.Delete(ctx, invite.Token); err != nil {
				g.renderError(c, http.StatusInternalServerError, err)
			} else {
				g.renderError(c, http.StatusPreconditionFailed, fmt.Errorf("this invitation has expired"))
			}
			return
		}

		dev, err := g.collections.Developers.GetByUsernameOrEmail(ctx, invite.EmailTo)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				if err := g.collections.Invites.Accept(ctx, invite.Token); err != nil {
					g.renderError(c, http.StatusInternalServerError, err)
				}
			} else {
				g.renderError(c, http.StatusInternalServerError, err)
				return
			}
		}
		if dev != nil {
			if err := g.collections.Orgs.AddMember(ctx, invite.Org, collections.Member{
				Key:      dev.Key,
				Username: dev.Username,
				Role:     collections.OrgMember,
			}); err != nil {
				if err == mongo.ErrNoDocuments {
					if err := g.collections.Invites.Delete(ctx, invite.Token); err != nil {
						g.renderError(c, http.StatusInternalServerError, err)

					} else {
						g.renderError(c, http.StatusNotFound, fmt.Errorf("org not found"))
					}
				} else {
					g.renderError(c, http.StatusInternalServerError, err)
				}
				return
			}
			if err = g.collections.Invites.Delete(ctx, invite.Token); err != nil {
				g.renderError(c, http.StatusInternalServerError, err)
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
func (g *Gateway) render404(c *gin.Context) {
	c.HTML(http.StatusNotFound, "/public/html/404.gohtml", nil)
}

// renderError renders the error template.
func (g *Gateway) renderError(c *gin.Context, code int, err error) {
	c.HTML(code, "/public/html/error.gohtml", gin.H{
		"Code":  code,
		"Error": formatError(err),
	})
}

// abort the request with code and error.
func abort(c *gin.Context, code int, err error) {
	c.AbortWithStatusJSON(code, gin.H{
		"error": err.Error(),
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
