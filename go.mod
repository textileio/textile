module github.com/textileio/textile

go 1.14

require (
	github.com/alecthomas/jsonschema v0.0.0-20191017121752-4bb6e3fae4f2
	github.com/caarlos0/spin v1.1.0
	github.com/cheggaaa/pb/v3 v3.0.4
	github.com/cloudflare/cloudflare-go v0.11.6
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/fatih/color v1.9.0 // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/gin-contrib/location v0.0.1
	github.com/gin-contrib/static v0.0.0-20191128031702-f81c604d8ac2
	github.com/gin-gonic/gin v1.6.2
	github.com/go-chi/chi v4.1.1+incompatible // indirect
	github.com/golang/protobuf v1.4.0
	github.com/gosimple/slug v1.9.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0
	github.com/improbable-eng/grpc-web v0.12.0
	github.com/ipfs/go-cid v0.0.6-0.20200501230655-7c82f3b81c00
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/go-ipfs-http-client v0.0.6-0.20200427093856-67f034a36a62
	github.com/ipfs/go-log v1.0.4
	github.com/ipfs/interface-go-ipfs-core v0.2.7
	github.com/jbenet/go-is-domain v1.0.3
	github.com/libp2p/go-libp2p-core v0.5.3
	github.com/logrusorgru/aurora v0.0.0-20200102142835-e9ef32dff381
	github.com/lunixbochs/vtclean v1.0.0 // indirect
	github.com/mailgun/mailgun-go/v3 v3.6.4
	github.com/mailru/easyjson v0.7.1 // indirect
	github.com/manifoldco/promptui v0.7.0
	github.com/mattn/go-colorable v0.1.6 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/mapstructure v1.3.0 // indirect
	github.com/multiformats/go-multiaddr v0.2.1
	github.com/multiformats/go-multibase v0.0.2
	github.com/olekukonko/tablewriter v0.0.4
	github.com/pelletier/go-toml v1.7.0 // indirect
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/rs/cors v1.7.0
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.6.3
	github.com/stretchr/testify v1.5.1
	github.com/textileio/go-assets v0.0.0-20200430191519-b341e634e2b7
	github.com/textileio/go-threads v0.1.17
	github.com/textileio/powergate v0.0.0-20200513174652-b60c2726e03a
	go.mongodb.org/mongo-driver v1.3.2
	go.uber.org/zap v1.15.0 // indirect
	golang.org/x/net v0.0.0-20200425230154-ff2c4b7c35a0 // indirect
	golang.org/x/sys v0.0.0-20200428200454-593003d681fa // indirect
	golang.org/x/time v0.0.0-20200416051211-89c76fbcd5d1 // indirect
	google.golang.org/genproto v0.0.0-20200428115010-c45acf45369a // indirect
	google.golang.org/grpc v1.29.1
	gopkg.in/ini.v1 v1.55.0 // indirect
)

replace github.com/ipfs/go-filestore => github.com/ipfs/go-filestore v1.0.0

//replace github.com/textileio/powergate => ../powergate
