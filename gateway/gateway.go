package gateway

import (
	"context"
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
	"github.com/google/uuid"
	logger "github.com/ipfs/go-log"
	assets "github.com/jessevdk/go-assets"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/rs/cors"
	gincors "github.com/rs/cors/wrapper/gin"
	"github.com/textileio/go-threads/broadcast"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/api/client"
	"github.com/textileio/textile/collections"
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
	addr        ma.Multiaddr
	url         string
	server      *http.Server
	collections *collections.Collections
	client      *client.Client
	clientAuth  client.Auth
	sessionBus  *broadcast.Broadcaster
}

// NewGateway returns a new gateway.
func NewGateway(addr ma.Multiaddr, url string, apiAddr ma.Multiaddr, apiToken string,
	collections *collections.Collections, sessionBus *broadcast.Broadcaster) (*Gateway, error) {
	apiTarget, err := util.TCPAddrFromMultiAddr(apiAddr)
	if err != nil {
		return nil, err
	}

	c, err := client.NewClient(apiTarget, nil)
	if err != nil {
		return nil, err
	}
	return &Gateway{
		addr:        addr,
		url:         url,
		collections: collections,
		client:      c,
		clientAuth:  client.Auth{Token: apiToken},
		sessionBus:  sessionBus,
	}, nil
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

	router.GET("/p/:project", g.bucketHandler)
	router.GET("/p/:project/*path", g.bucketHandler)

	router.POST("/register", g.registerAppUser)

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

// APIToken returns the gateway's internal API token.
func (g *Gateway) APIToken() string {
	return g.clientAuth.Token
}

// Stop the gateway.
func (g *Gateway) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := g.server.Shutdown(ctx); err != nil {
		log.Errorf("error shutting down gateway: %s", err)
		return err
	}
	g.client.Close()
	return nil
}

// confirmEmail verifies an emailed secret.
func (g *Gateway) confirmEmail(c *gin.Context) {
	if err := g.sessionBus.Send(g.parseUUID(c, c.Param("secret"))); err != nil {
		g.renderError(c, http.StatusInternalServerError, err)
		return
	}

	c.HTML(http.StatusOK, "/public/html/confirm.gohtml", nil)
}

// consentInvite adds a user to a team.
func (g *Gateway) consentInvite(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	invite, err := g.collections.Invites.Get(ctx, g.parseUUID(c, c.Param("invite")))
	if err != nil {
		g.render404(c)
		return
	}
	if invite.Expiry < int(time.Now().Unix()) {
		g.renderError(c, http.StatusPreconditionFailed, fmt.Errorf("this invitation has expired"))
		return
	}

	matches, err := g.collections.Users.GetByEmail(ctx, invite.ToEmail)
	if err != nil {
		g.renderError(c, http.StatusInternalServerError, err)
		return
	}
	var user *collections.User
	if len(matches) == 0 {
		user, err = g.collections.Users.Create(ctx, invite.ToEmail)
		if err != nil {
			g.renderError(c, http.StatusInternalServerError, err)
			return
		}
	} else {
		user = matches[0]
	}

	team, err := g.collections.Teams.Get(ctx, invite.TeamID)
	if err != nil {
		g.render404(c)
		return
	}
	if err = g.collections.Users.JoinTeam(ctx, user, team.ID); err != nil {
		g.renderError(c, http.StatusInternalServerError, err)
		return
	}

	c.HTML(http.StatusOK, "/public/html/consent.gohtml", gin.H{
		"Team": team.Name,
	})
}

type link struct {
	Name  string
	Path  string
	Size  string
	Links string
}

// bucketHandler renders bucket files and directories.
func (g *Gateway) bucketHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	projectID := c.Param("project")
	rep, err := g.client.ListBucketPath(ctx, g.parseUUID(c, projectID), c.Param("path"), g.clientAuth)
	if err != nil {
		abort(c, http.StatusNotFound, fmt.Errorf("project not found"))
		return
	}

	if !rep.Item.IsDir {
		if err := g.client.PullBucketPath(ctx, c.Param("path"), c.Writer, g.clientAuth); err != nil {
			abort(c, http.StatusInternalServerError, err)
		}
	} else {
		projectPath := path.Join("p", projectID)

		links := make([]link, len(rep.Item.Items))
		for i, item := range rep.Item.Items {
			var pth string
			if rep.Root != nil {
				pth = strings.Replace(item.Path, rep.Root.Path, rep.Root.Name, 1)
			} else {
				pth = item.Name
			}

			if rep.Root != nil && item.Name == "index.html" {
				if err := g.client.PullBucketPath(ctx, pth, c.Writer, g.clientAuth); err != nil {
					abort(c, http.StatusInternalServerError, err)
				}
				return
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
		c.HTML(http.StatusOK, "/public/html/bucket.gohtml", gin.H{
			"Path":  rep.Item.Path,
			"Root":  root,
			"Back":  back,
			"Links": links,
		})
	}
}

type registrationParams struct {
	Token    string `json:"token" binding:"required"`
	DeviceID string `json:"device_id" binding:"required"`
}

// registerAppUser adds a user to a team.
func (g *Gateway) registerAppUser(c *gin.Context) {
	var params registrationParams
	err := c.BindJSON(&params)
	if err != nil {
		abort(c, http.StatusBadRequest, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	token, err := g.collections.Tokens.Get(ctx, params.Token)
	if err != nil {
		abort(c, http.StatusNotFound, fmt.Errorf("token not found"))
		return
	}
	proj, err := g.collections.Projects.Get(ctx, token.ProjectID)
	if err != nil {
		abort(c, http.StatusNotFound, fmt.Errorf("project not found"))
		return
	}
	user, err := g.collections.AppUsers.GetOrCreate(ctx, proj.ID, params.DeviceID)
	if err != nil {
		abort(c, http.StatusInternalServerError, err)
		return
	}

	session, err := g.collections.Sessions.Create(ctx, user.ID, user.ID)
	if err != nil {
		abort(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         user.ID,
		"session_id": session.ID,
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

// parseUUID parses a string as a UUID, adding back hyphens.
func (g *Gateway) parseUUID(c *gin.Context, param string) (parsed string) {
	id, err := uuid.Parse(param)
	if err != nil {
		g.render404(c)
		return
	}
	return id.String()
}
