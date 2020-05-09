package gateway

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	gopath "path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	ipath "github.com/ipfs/go-path"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/path"
	isd "github.com/jbenet/go-is-domain"
	"github.com/libp2p/go-libp2p-core/peer"
	mbase "github.com/multiformats/go-multibase"
)

// ipfsNamespaceHandler redirects IPFS namespaces to their subdomain equivalents.
func (g *Gateway) ipfsNamespaceHandler(c *gin.Context) {
	loc, ok := g.toSubdomainURL(c.Request)
	if !ok {
		render404(c)
		return
	}

	// See security note https://github.com/ipfs/go-ipfs/blob/dbfa7bf2b216bad9bec1ff66b1f3814f4faac31e/core/corehttp/hostname.go#L105
	c.Request.Header.Set("Clear-Site-Data", "\"cookies\", \"storage\"")

	c.Redirect(http.StatusPermanentRedirect, loc)
}

func (g *Gateway) renderIPFSPath(c *gin.Context, ctx context.Context, pth path.Path) {
	data, err := g.openPath(ctx, pth)
	if err != nil {
		if err == iface.ErrIsDir {
			root, err := ipath.ParsePath(strings.TrimSuffix(pth.String(), "/"))
			if err != nil {
				renderError(c, http.StatusInternalServerError, err)
				return
			}
			var fpath, back string
			parts := strings.Split(root.String(), "/")
			if len(parts) > 2 {
				fpath = strings.Join(parts[3:], "/")
				back = gopath.Dir(fpath)
			}
			if fpath == "" {
				back = ""
			}
			lctx, lcancel := context.WithTimeout(context.Background(), handlerTimeout)
			defer lcancel()
			ilinks, err := g.ipfs.Object().Links(lctx, pth)
			if err != nil {
				renderError(c, http.StatusInternalServerError, err)
				return
			}
			var links []link
			for _, l := range ilinks {
				links = append(links, link{
					Name: l.Name,
					Path: gopath.Join(fpath, l.Name),
					Size: byteCountDecimal(int64(l.Size)),
				})
			}
			c.HTML(http.StatusOK, "/public/html/unixfs.gohtml", gin.H{
				"Title":   "Index of " + root.String(),
				"Root":    "",
				"Path":    root.String(),
				"Updated": "",
				"Back":    strings.TrimPrefix(back, "/"),
				"Links":   links,
			})
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

// Copied from https://github.com/ipfs/go-ipfs/blob/dbfa7bf2b216bad9bec1ff66b1f3814f4faac31e/core/corehttp/hostname.go#L251
func isSubdomainNamespace(ns string) bool {
	switch ns {
	case "ipfs", "ipns", "p2p", "ipld":
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

	urlp := strings.Split(g.url, "://")
	scheme := urlp[0]
	host := urlp[1]
	return safeRedirectURL(fmt.Sprintf("%s://%s.%s.%s/%s%s", scheme, rootID, ns, host, rest, query))
}

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
