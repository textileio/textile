module github.com/textileio/textile/v2

go 1.15

require (
	github.com/alecthomas/jsonschema v0.0.0-20191017121752-4bb6e3fae4f2
	github.com/blang/semver v3.5.1+incompatible
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/caarlos0/spin v1.1.0
	github.com/cenkalti/backoff/v4 v4.1.1
	github.com/cheggaaa/pb/v3 v3.0.5
	github.com/cloudflare/cloudflare-go v0.11.6
	github.com/customerio/go-customerio v2.0.0+incompatible
	github.com/dustin/go-humanize v1.0.0
	github.com/filecoin-project/go-fil-markets v1.1.9
	github.com/gin-contrib/location v0.0.2
	github.com/gin-contrib/pprof v1.3.0
	github.com/gin-contrib/static v0.0.1
	github.com/gin-gonic/gin v1.9.0
	github.com/gogo/status v1.1.0
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.5
	github.com/gosimple/slug v1.9.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.2.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/improbable-eng/grpc-web v0.14.1
	github.com/ipfs/go-blockservice v0.1.4
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-datastore v0.4.5
	github.com/ipfs/go-ds-flatfs v0.4.4
	github.com/ipfs/go-ipfs-blockstore v1.0.4
	github.com/ipfs/go-ipfs-chunker v0.0.5
	github.com/ipfs/go-ipfs-ds-help v1.0.0
	github.com/ipfs/go-ipfs-exchange-offline v0.0.1
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/go-ipfs-http-client v0.1.0
	github.com/ipfs/go-ipld-cbor v0.0.5
	github.com/ipfs/go-ipld-format v0.2.0
	github.com/ipfs/go-log/v2 v2.3.0
	github.com/ipfs/go-merkledag v0.3.2
	github.com/ipfs/go-unixfs v0.2.6
	github.com/ipfs/interface-go-ipfs-core v0.4.0
	github.com/jbenet/go-is-domain v1.0.3
	github.com/jhump/protoreflect v1.7.0
	github.com/launchdarkly/go-country-codes v0.0.0-20191008001159-776cf5214f39
	github.com/libp2p/go-libp2p-core v0.8.6
	github.com/logrusorgru/aurora v0.0.0-20200102142835-e9ef32dff381
	github.com/manifoldco/promptui v0.7.0
	github.com/mattn/go-colorable v0.1.8 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/mapstructure v1.3.0 // indirect
	github.com/multiformats/go-multiaddr v0.3.3
	github.com/multiformats/go-multibase v0.0.3
	github.com/multiformats/go-multihash v0.0.15
	github.com/oklog/ulid/v2 v2.0.2
	github.com/olekukonko/tablewriter v0.0.5
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/radovskyb/watcher v1.0.7
	github.com/rhysd/go-github-selfupdate v1.2.2
	github.com/robfig/cron/v3 v3.0.1
	github.com/rs/cors v1.7.0
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06
	github.com/segmentio/backo-go v0.0.0-20200129164019-23eae7c10bd3 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.8.1
	github.com/stripe/stripe-go/v72 v72.10.0
	github.com/tchap/go-patricia v2.3.0+incompatible // indirect
	github.com/textileio/crypto v0.0.0-20210929130053-08edebc3361a
	github.com/textileio/dcrypto v0.0.1
	github.com/textileio/go-assets v0.0.0-20200430191519-b341e634e2b7
	github.com/textileio/go-ds-mongo v0.1.5
	github.com/textileio/go-threads v1.1.6-0.20220409162902-a715c2402413
	github.com/textileio/powergate/v2 v2.3.0
	github.com/textileio/swagger-ui v0.3.29-0.20210224180244-7d73a7a32fe7
	github.com/xakep666/mongo-migrate v0.2.1
	github.com/xtgo/uuid v0.0.0-20140804021211-a0b114877d4c // indirect
	go.mongodb.org/mongo-driver v1.8.1
	golang.org/x/net v0.7.0
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4
	golang.org/x/text v0.7.0
	golang.org/x/time v0.0.0-20200416051211-89c76fbcd5d1
	google.golang.org/genproto v0.0.0-20210207032614-bba0dbe2a9ea
	google.golang.org/grpc v1.39.0
	google.golang.org/protobuf v1.28.1
	gopkg.in/ini.v1 v1.55.0 // indirect
	gopkg.in/segmentio/analytics-go.v3 v3.1.0
)

replace github.com/ipfs/go-ipns => github.com/ipfs/go-ipns v0.0.2

replace github.com/improbable-eng/grpc-web v0.14.1 => github.com/jsmouret/grpc-web v0.14.2-0.20211103063242-8c932b2237aa
