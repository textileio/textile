module github.com/textileio/textile

go 1.14

replace github.com/ipfs/go-datastore v0.4.4 => github.com/textileio/go-datastore v0.4.5-0.20200728205504-ffeb3591b248

replace github.com/ipfs/go-ds-badger v0.2.4 => github.com/textileio/go-ds-badger v0.2.5-0.20200728212847-1ec9ac5e644c

require (
	github.com/AndreasBriese/bbloom v0.0.0-20190825152654-46b345b51c96 // indirect
	github.com/alecthomas/jsonschema v0.0.0-20191017121752-4bb6e3fae4f2
	github.com/caarlos0/spin v1.1.0
	github.com/cenkalti/backoff/v4 v4.0.2
	github.com/cloudflare/cloudflare-go v0.11.6
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/gin-contrib/location v0.0.2
	github.com/gin-contrib/static v0.0.0-20191128031702-f81c604d8ac2
	github.com/gin-gonic/gin v1.6.3
	github.com/go-chi/chi v4.1.1+incompatible // indirect
	github.com/gogo/status v1.1.0
	github.com/golang/protobuf v1.4.2
	github.com/gosimple/slug v1.9.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0
	github.com/hsanjuan/ipfs-lite v1.1.12 // indirect
	github.com/improbable-eng/grpc-web v0.13.0
	github.com/ipfs/go-blockservice v0.1.4-0.20200624145336-a978cec6e834
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-datastore v0.4.4
	github.com/ipfs/go-ds-flatfs v0.4.4
	github.com/ipfs/go-ipfs-blockstore v1.0.0
	github.com/ipfs/go-ipfs-chunker v0.0.5
	github.com/ipfs/go-ipfs-ds-help v1.0.0
	github.com/ipfs/go-ipfs-exchange-offline v0.0.1
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/go-ipfs-http-client v0.0.6-0.20200512220018-7002cce28cb1
	github.com/ipfs/go-ipld-cbor v0.0.5-0.20200428170625-a0bd04d3cbdf
	github.com/ipfs/go-ipld-format v0.2.0
	github.com/ipfs/go-log v1.0.4
	github.com/ipfs/go-merkledag v0.3.2
	github.com/ipfs/go-unixfs v0.2.4
	github.com/ipfs/interface-go-ipfs-core v0.2.7
	github.com/jbenet/go-is-domain v1.0.3
	github.com/jhump/protoreflect v1.7.0
	github.com/libp2p/go-libp2p-core v0.6.1
	github.com/libp2p/go-libp2p-gostream v0.2.1 // indirect
	github.com/logrusorgru/aurora v0.0.0-20200102142835-e9ef32dff381
	github.com/mailgun/mailgun-go/v3 v3.6.4
	github.com/mailru/easyjson v0.7.1 // indirect
	github.com/manifoldco/promptui v0.7.0
	github.com/mattn/go-colorable v0.1.6 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/mapstructure v1.3.0 // indirect
	github.com/multiformats/go-multiaddr v0.2.2
	github.com/multiformats/go-multibase v0.0.3
	github.com/multiformats/go-multihash v0.0.14
	github.com/oklog/ulid/v2 v2.0.2
	github.com/olekukonko/tablewriter v0.0.4
	github.com/pelletier/go-toml v1.7.0 // indirect
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/radovskyb/watcher v1.0.7
	github.com/rs/cors v1.7.0
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.6.1
	github.com/textileio/dcrypto v0.0.1
	github.com/textileio/go-assets v0.0.0-20200430191519-b341e634e2b7
	github.com/textileio/go-threads v0.1.24-0.20200728224844-456a1ebdf635
	github.com/textileio/powergate v0.4.0
	github.com/textileio/uiprogress v0.0.4
	go.mongodb.org/mongo-driver v1.3.2
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	golang.org/x/time v0.0.0-20200416051211-89c76fbcd5d1 // indirect
	golang.org/x/tools v0.0.0-20200522201501-cb1345f3a375 // indirect
	google.golang.org/grpc v1.31.0
	gopkg.in/ini.v1 v1.55.0 // indirect
	honnef.co/go/tools v0.0.1-2020.1.4 // indirect
)

// Fixes races. Keep this until this gets tagged and propagated to other deps.
replace github.com/libp2p/go-libp2p-peerstore => github.com/libp2p/go-libp2p-peerstore v0.2.5-0.20200605182041-9827ee08601f
