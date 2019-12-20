package gateway

import (
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	logger "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/rs/cors"
	gincors "github.com/rs/cors/wrapper/gin"
	"github.com/textileio/go-textile-core/broadcast"
	"github.com/textileio/textile/gateway/static/css"
	"github.com/textileio/textile/gateway/templates"
)

var log = logger.Logger("gateway")

// Host is the instance used by the daemon
// var Host *Gateway // @todo: still best to run it this way?

// Gateway is a HTTP API for getting files and links from IPFS
type Gateway struct {
	addr   ma.Multiaddr
	bus    *broadcast.Broadcaster
	server *http.Server
}

type Config struct {
	GatewayAddr ma.Multiaddr
}

// NewGateway returns a new gateway
func NewGateway(conf Config) *Gateway {
	bus := broadcast.NewBroadcaster(0)
	return &Gateway{
		addr: conf.GatewayAddr,
		bus:  bus,
	}
}

// Start creates a gateway server
func (g *Gateway) Start() {
	loc, err := parseFromAddr(g.addr)
	if err != nil {
		log.Fatal(err)
	}

	gin.SetMode(gin.ReleaseMode)

	// @todo: conf based headers
	options := cors.Options{}

	router := gin.Default()
	router.Use(location.Default())

	// Add the CORS middleware
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
		Addr:    loc,
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
				if err != nil && err != http.ErrServerClosed {
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

// Addr returns the gateway's address
func (g *Gateway) Addr() string {
	return g.server.Addr
}

// Bus returns the broadcast bus instance
func (g *Gateway) Bus() *broadcast.Broadcaster {
	return g.bus
}

// Stop stops the gateway
func (g *Gateway) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := g.server.Shutdown(ctx); err != nil {
		log.Errorf("error shutting down gateway: %s", err)
		return err
	}
	g.bus.Discard()
	return nil
}

// emailVerification accepts a secret to verify
func (g *Gateway) emailVerification(c *gin.Context) {
	secret := c.Param("secret")
	var data []byte
	if secret != "" {
		data = []byte("Success")
		g.bus.Send(secret)
	} else {
		data = []byte("Error")
	}
	c.Render(200, render.Data{Data: data})
}

// render404 renders the 404 template
func (g *Gateway) render404(c *gin.Context) {
	if strings.Contains(c.Request.URL.String(), "small/content") ||
		strings.Contains(c.Request.URL.String(), "large/content") {
		var url string

		loc := location.Get(c)
		url = fmt.Sprintf("%s://%s", loc.Scheme, loc.Host)

		pth := strings.Replace(c.Request.URL.String(), "/content", "/d", 1)
		c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("%s%s", url, pth))
		return
	}

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

func parseFromAddr(addr ma.Multiaddr) (string, error) {
	parts := strings.Split(addr.String(), "/")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid gateway address")
	}
	parsed := net.ParseIP(parts[2])
	if parsed == nil {
		return "", fmt.Errorf("unsupported multiformat")
	}
	if len(parts) < 5 {
		return parsed.String(), nil
	}
	if parts[3] == "tcp" {
		return fmt.Sprintf("%s:%s", parsed.String(), parts[4]), nil
	}
	return "", fmt.Errorf("no address was found")
}
