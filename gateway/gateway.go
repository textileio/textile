package gateway

import (
	"context"
	"encoding/base64"
	"html/template"
	"net/http"
	"time"

	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	logger "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/rs/cors"
	gincors "github.com/rs/cors/wrapper/gin"
	"github.com/textileio/go-textile-core/broadcast"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/gateway/static/css"
	"github.com/textileio/textile/gateway/templates"
)

var log = logger.Logger("gateway")

// Gateway provides HTTP-based access to Textile.
type Gateway struct {
	addr       ma.Multiaddr
	url        string
	server     *http.Server
	sessionBus *broadcast.Broadcaster
}

// NewGateway returns a new gateway.
func NewGateway(addr ma.Multiaddr, url string) *Gateway {
	return &Gateway{
		addr:       addr,
		url:        url,
		sessionBus: broadcast.NewBroadcaster(0),
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

	router.SetHTMLTemplate(parseTemplates())

	router.GET("/health", func(c *gin.Context) {
		c.Writer.WriteHeader(http.StatusNoContent)
	})
	router.GET("/favicon.ico", func(c *gin.Context) {
		img, err := base64.StdEncoding.DecodeString(favicon)
		if err != nil {
			c.Writer.WriteHeader(http.StatusNotFound)
			return
		}
		c.Header("Cache-Control", "public, max-age=172800")
		c.Render(http.StatusOK, render.Data{Data: img})
	})
	router.GET("/static/css/style.css", func(c *gin.Context) {
		c.Header("Content-Type", "text/css; charset=utf-8")
		c.Header("Cache-Control", "public, max-age=172800")
		c.String(http.StatusOK, css.Style)
	})

	router.GET("/verify/:secret", g.emailVerification)

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

// emailVerification accepts a secret to verify
func (g *Gateway) emailVerification(c *gin.Context) {
	secret := c.Param("secret")
	if secret != "" {
		if err := g.sessionBus.Send(secret); err != nil {
			c.Render(http.StatusInternalServerError, render.Data{
				Data: []byte(err.Error()),
			})
		} else {
			c.Render(http.StatusOK, render.Data{
				Data: []byte("Success"),
			})
		}
	} else {
		c.Render(http.StatusBadRequest, render.Data{
			Data: []byte("Bad request"),
		})
	}
}

// render404 renders the 404 template
func (g *Gateway) render404(c *gin.Context) {
	c.HTML(http.StatusNotFound, "404", nil)
}

// parseTemplates loads HTML templates
func parseTemplates() *template.Template {
	temp, err := template.New("index").Parse(templates.Index)
	if err != nil {
		panic(err)
	}
	temp, err = temp.New("404").Parse(templates.NotFound)
	if err != nil {
		panic(err)
	}
	return temp
}
