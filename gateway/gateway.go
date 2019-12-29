package gateway

import (
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/location"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	logger "github.com/ipfs/go-log"
	assets "github.com/jessevdk/go-assets"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/rs/cors"
	gincors "github.com/rs/cors/wrapper/gin"
	"github.com/textileio/go-textile-core/broadcast"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/collections"
)

var log = logger.Logger("gateway")

// fileSystem extends the binary asset file system with Exists,
// enabling its use with the static middleware.
type fileSystem struct {
	*assets.FileSystem
}

// Exists returns whether or not the path exists in the binary assets.
func (f *fileSystem) Exists(prefix, path string) bool {
	_, ok := f.Files[strings.TrimPrefix(path, prefix)]
	return ok
}

// Gateway provides HTTP-based access to Textile.
type Gateway struct {
	addr        ma.Multiaddr
	url         string
	server      *http.Server
	collections *collections.Collections
	sessionBus  *broadcast.Broadcaster
}

// NewGateway returns a new gateway.
func NewGateway(addr ma.Multiaddr, url string, collections *collections.Collections) *Gateway {
	return &Gateway{
		addr:        addr,
		url:         url,
		collections: collections,
		sessionBus:  broadcast.NewBroadcaster(0),
	}
}

// Start the gateway.
func (g *Gateway) Start() {
	addr, err := util.TCPAddrFromMultiAddr(g.addr)
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

	router.GET("/health", func(c *gin.Context) {
		c.Writer.WriteHeader(http.StatusNoContent)
	})

	router.GET("/confirm/:secret", g.confirmEmail)
	router.GET("/consent/:invite", g.consentInvite)

	router.NoRoute(func(c *gin.Context) {
		g.render404(c)
	})

	g.server = &http.Server{
		Addr:    addr,
		Handler: router,
	}

	errc := make(chan error)
	go func() {
		errc <- g.server.ListenAndServe()
		close(errc)
	}()
	go func() {
		for {
			select {
			case err, ok := <-errc:
				if err != nil {
					if err == http.ErrServerClosed {
						return
					}
					log.Errorf("gateway error: %s", err)
				}
				if !ok {
					log.Info("gateway was shutdown")
					return
				}
			}
		}
	}()
	log.Infof("gateway listening at %s", g.server.Addr)
}

// Addr returns the gateway's address.
func (g *Gateway) Addr() string {
	return g.server.Addr
}

// Url returns the gateway's public facing URL.
func (g *Gateway) Url() string {
	return g.url
}

// SessionListener returns a listener used for session verification.
func (g *Gateway) SessionListener() *broadcast.Listener {
	return g.sessionBus.Listen()
}

// Stop the gateway.
func (g *Gateway) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := g.server.Shutdown(ctx); err != nil {
		log.Errorf("error shutting down gateway: %s", err)
		return err
	}
	g.sessionBus.Discard()
	return nil
}

// confirmEmail verifies an emailed secret.
func (g *Gateway) confirmEmail(c *gin.Context) {
	secret := c.Param("secret")
	if secret == "" {
		g.render404(c)
		return
	}
	if err := g.sessionBus.Send(secret); err != nil {
		g.renderError(c, http.StatusInternalServerError, err)
		return
	}

	c.HTML(http.StatusOK, "/public/html/confirm.gohtml", nil)
}

// consentInvite adds a user to a team.
func (g *Gateway) consentInvite(c *gin.Context) {
	inviteID := c.Param("invite")
	if inviteID == "" {
		g.render404(c)
		return
	}
	invite, err := g.collections.Invites.Get(inviteID)
	if err != nil {
		g.render404(c)
		return
	}
	if invite.Expiry < int(time.Now().Unix()) {
		g.renderError(c, http.StatusPreconditionFailed, fmt.Errorf("this invitation has expired"))
		return
	}

	matches, err := g.collections.Users.GetByEmail(invite.ToEmail)
	if err != nil {
		g.renderError(c, http.StatusInternalServerError, err)
		return
	}
	var user *collections.User
	if len(matches) == 0 {
		user, err = g.collections.Users.Create(invite.ToEmail)
		if err != nil {
			g.renderError(c, http.StatusInternalServerError, err)
			return
		}
	} else {
		user = matches[0]
	}

	team, err := g.collections.Teams.Get(invite.TeamID)
	if err != nil {
		g.render404(c)
		return
	}
	if err = g.collections.Users.JoinTeam(user, team.ID); err != nil {
		g.renderError(c, http.StatusInternalServerError, err)
		return
	}

	c.HTML(http.StatusOK, "/public/html/consent.gohtml", gin.H{
		"Team": team.Name,
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

func formatError(err error) string {
	words := strings.SplitN(err.Error(), " ", 2)
	words[0] = strings.Title(words[0])
	return strings.Join(words, " ") + "."
}
