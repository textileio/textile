package gateway

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	gopath "path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	files "github.com/ipfs/go-ipfs-files"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/peer"
)

func (g *Gateway) ipfsHandler(c *gin.Context) {
	base := fmt.Sprintf("ipfs/%s", c.Param("root"))
	pth := fmt.Sprintf("/%s%s", base, c.Param("path"))
	g.renderIPFSPath(c, base, pth)
}

func (g *Gateway) renderIPFSPath(c *gin.Context, base, pth string) {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()
	pth = strings.TrimSuffix(pth, "/")
	data, err := g.openPath(ctx, path.New(pth))
	if err != nil {
		if err == iface.ErrIsDir {
			var root, dir, back string
			parts := strings.Split(pth, "/")
			if len(parts) > 2 {
				root = strings.Join(parts[:3], "/")
				dir = strings.Join(parts[3:], "/")
				back = gopath.Dir(dir)
			}
			if dir == "" {
				back = ""
			}
			if !g.subdomains {
				dir = gopath.Join(base, dir)
				if back != "" {
					back = gopath.Join(base, back)
				}
			}
			lctx, lcancel := context.WithTimeout(context.Background(), handlerTimeout)
			defer lcancel()
			ilinks, err := g.ipfs.Object().Links(lctx, path.New(pth))
			if err != nil {
				renderError(c, http.StatusNotFound, err)
				return
			}
			var links []link
			for _, l := range ilinks {
				links = append(links, link{
					Name: l.Name,
					Path: gopath.Join(dir, l.Name),
					Size: byteCountDecimal(int64(l.Size)),
				})
			}
			var index string
			if strings.HasPrefix(base, "ipns") {
				index = gopath.Join("/", base, dir)
			} else {
				index = gopath.Join(root, dir)
			}
			if !g.subdomains {
				dir = strings.TrimPrefix(strings.Replace(dir, base, "", 1), "/")
			}
			params := gin.H{
				"Title":   "Index of " + index,
				"Root":    "/" + dir,
				"Path":    pth,
				"Updated": "",
				"Back":    strings.TrimPrefix(back, "/"),
				"Links":   links,
			}
			c.HTML(http.StatusOK, "/public/html/unixfs.gohtml", params)
		} else {
			renderError(c, http.StatusBadRequest, err)
			return
		}
	} else {
		c.Render(200, render.Data{Data: data})
	}
}

func (g *Gateway) openPath(ctx context.Context, pth path.Path) ([]byte, error) {
	f, err := g.ipfs.Unixfs().Get(ctx, pth)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var file files.File
	switch f := f.(type) {
	case files.File:
		file = f
	case files.Directory:
		return nil, iface.ErrIsDir
	default:
		return nil, iface.ErrNotSupported
	}
	return ioutil.ReadAll(file)
}

func (g *Gateway) ipnsHandler(c *gin.Context) {
	g.renderIPNSKey(c, c.Param("key"), c.Param("path"))
}

func (g *Gateway) renderIPNSKey(c *gin.Context, key, pth string) {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()
	root, err := g.ipfs.Name().Resolve(ctx, key)
	if err != nil {
		renderError(c, http.StatusNotFound, err)
		return
	}
	base := fmt.Sprintf("ipns/%s", key)
	g.renderIPFSPath(c, base, gopath.Join(root.String(), pth))
}

func (g *Gateway) p2pHandler(c *gin.Context) {
	g.renderP2PKey(c, c.Param("key"))
}

func (g *Gateway) renderP2PKey(c *gin.Context, key string) {
	pid, err := peer.Decode(key)
	if err != nil {
		renderError(c, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()
	info, err := g.ipfs.Dht().FindPeer(ctx, pid)
	if err != nil {
		renderError(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

func (g *Gateway) ipldHandler(c *gin.Context) {
	pth := fmt.Sprintf("%s%s", c.Param("root"), strings.TrimSuffix(c.Param("path"), "/"))
	g.renderP2PKey(c, pth)
}

func (g *Gateway) renderIPLDPath(c *gin.Context, pth string) {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()
	node, err := g.ipfs.Object().Get(ctx, path.New(pth))
	if err != nil {
		renderError(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, node)
}
