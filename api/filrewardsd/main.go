package main

import (
	"encoding/json"
	"fmt"
	"net"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/go-threads/util"
	pow "github.com/textileio/powergate/v2/api/client"
	analytics "github.com/textileio/textile/v2/api/analyticsd/client"
	"github.com/textileio/textile/v2/api/filrewardsd/service"
	"github.com/textileio/textile/v2/api/filrewardsd/service/claimstore"
	"github.com/textileio/textile/v2/api/filrewardsd/service/rewardstore"
	sendfil "github.com/textileio/textile/v2/api/sendfild/client"
	"github.com/textileio/textile/v2/cmd"
	mdb "github.com/textileio/textile/v2/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
)

const daemonName = "filrewardsd"

var (
	log = logging.Logger(daemonName)

	config = &cmd.Config{
		Viper: viper.New(),
		Dir:   "." + daemonName,
		Name:  "config",
		Flags: map[string]cmd.Flag{
			"debug": {
				Key:      "debug",
				DefValue: false,
			},
			"logFile": {
				Key:      "log_file",
				DefValue: "", // no log file
			},
			"listenAddr": {
				Key:      "listen_addr",
				DefValue: "127.0.0.1:5000",
			},
			"mongoUri": {
				Key:      "mongo_uri",
				DefValue: "mongodb://127.0.0.1:27017",
			},
			"mongoDb": {
				Key:      "mongo_db",
				DefValue: "textile_filrewards",
			},
			"mongoAccountsDb": {
				Key:      "mongo_accounts_db",
				DefValue: "textile",
			},
			"analyticsAddr": {
				Key:      "analytics_addr",
				DefValue: "",
			},
			"sendfilAddr": {
				Key:      "sendfil_addr",
				DefValue: "",
			},
			"powAddr": {
				Key:      "pow_addr",
				DefValue: "",
			},
			"fundingAddr": {
				Key:      "funding_addr",
				DefValue: "",
			},
			"isDevnet": {
				Key:      "is_devnet",
				DefValue: false,
			},
			"baseNanoFilReward": {
				Key:      "base_nano_fil_reward",
				DefValue: int64(0),
			},
		},
		EnvPre: "FILREWARDS",
		Global: true,
	}
)

func init() {
	cobra.OnInitialize(cmd.InitConfig(config))
	cmd.InitConfigCmd(rootCmd, config.Viper, config.Dir)

	rootCmd.PersistentFlags().StringVar(
		&config.File,
		"config",
		"",
		"Config file (default ${HOME}/"+config.Dir+"/"+config.Name+".yml)")
	rootCmd.PersistentFlags().BoolP(
		"debug",
		"d",
		config.Flags["debug"].DefValue.(bool),
		"Enable debug logging")
	rootCmd.PersistentFlags().String(
		"logFile",
		config.Flags["logFile"].DefValue.(string),
		"Write logs to file")

	rootCmd.PersistentFlags().String(
		"listenAddr",
		config.Flags["listenAddr"].DefValue.(string),
		"Filrewards API listen address")

	rootCmd.PersistentFlags().String(
		"mongoUri",
		config.Flags["mongoUri"].DefValue.(string),
		"MongoDB connection URI")
	rootCmd.PersistentFlags().String(
		"mongoDb",
		config.Flags["mongoDb"].DefValue.(string),
		"MongoDB database name")
	rootCmd.PersistentFlags().String(
		"mongoAccountsDb",
		config.Flags["mongoAccountsDb"].DefValue.(string),
		"MongoDB accounts database name")

	rootCmd.PersistentFlags().String(
		"analyticsAddr",
		config.Flags["analyticsAddr"].DefValue.(string),
		"Analytics API address")
	rootCmd.PersistentFlags().String(
		"sendfilAddr",
		config.Flags["sendfilAddr"].DefValue.(string),
		"Sendfil API address")
	rootCmd.PersistentFlags().String(
		"powAddr",
		config.Flags["powAddr"].DefValue.(string),
		"Powergate API address")

	rootCmd.PersistentFlags().String(
		"fundingAddr",
		config.Flags["fundingAddr"].DefValue.(string),
		"The Filecoin address used to fund claims")

	rootCmd.PersistentFlags().Bool(
		"isDevnet",
		config.Flags["isDevnet"].DefValue.(bool),
		"Whether or not we're running lotus-devnet")

	rootCmd.PersistentFlags().Int64(
		"baseNanoFilReward",
		config.Flags["baseNanoFilReward"].DefValue.(int64),
		"Base NanoFIL reward amount",
	)

	err := cmd.BindFlags(config.Viper, rootCmd, config.Flags)
	cmd.ErrCheck(err)
}

func main() {
	cmd.ErrCheck(rootCmd.Execute())
}

var rootCmd = &cobra.Command{
	Use:   daemonName,
	Short: "Filrewards daemon",
	Long:  `Textile's filrewards daemon.`,
	PersistentPreRun: func(c *cobra.Command, args []string) {
		config.Viper.SetConfigType("yaml")
		cmd.ExpandConfigVars(config.Viper, config.Flags)

		if config.Viper.GetBool("debug") {
			err := util.SetLogLevels(map[string]logging.LogLevel{
				daemonName: logging.LevelDebug,
			})
			cmd.ErrCheck(err)
		}
	},
	Run: func(c *cobra.Command, args []string) {
		settings, err := json.MarshalIndent(config.Viper.AllSettings(), "", "  ")
		cmd.ErrCheck(err)
		log.Debugf("loaded config: %s", string(settings))

		debug := config.Viper.GetBool("debug")
		logFile := config.Viper.GetString("log_file")
		listenAddr := config.Viper.GetString("listen_addr")
		mongoUri := config.Viper.GetString("mongo_uri")
		mongoDb := config.Viper.GetString("mongo_db")
		mongoAccountsDb := config.Viper.GetString("mongo_accounts_db")
		analyticsAddr := config.Viper.GetString("analytics_addr")
		sendfilAddr := config.Viper.GetString("sendfil_addr")
		powAddr := config.Viper.GetString("pow_addr")
		fundingAddr := config.Viper.GetString("funding_addr")
		isDevnet := config.Viper.GetBool("is_devnet")
		baseFilReward := config.Viper.GetInt64("base_nano_fil_reward")

		if logFile != "" {
			err = cmd.SetupDefaultLoggingConfig(logFile)
			cmd.ErrCheck(err)
		}

		listener, err := net.Listen("tcp", listenAddr)
		cmd.ErrCheck(err)

		mongoClient, err := mongo.Connect(c.Context(), options.Client().ApplyURI(mongoUri))
		cmd.ErrCheck(err)
		db := mongoClient.Database(mongoDb)
		accountsDb := mongoClient.Database(mongoAccountsDb)

		rs, err := rewardstore.New(db, debug)
		cmd.ErrCheck(err)

		cs, err := claimstore.New(db, debug)
		cmd.ErrCheck(err)

		as, err := mdb.NewAccounts(c.Context(), accountsDb)
		cmd.ErrCheck(err)

		var ac *analytics.Client
		if analyticsAddr != "" {
			ac, err = analytics.New(analyticsAddr, grpc.WithInsecure())
			cmd.ErrCheck(err)
		}

		sendfilConn, err := grpc.Dial(sendfilAddr, grpc.WithInsecure())
		cmd.ErrCheck(err)
		sc, err := sendfil.New(sendfilConn)
		cmd.ErrCheck(err)

		pc, err := pow.NewClient(powAddr, grpc.WithInsecure(), grpc.WithBlock())
		cmd.ErrCheck(err)

		if isDevnet {
			res, err := pc.Admin.Wallet.Addresses(c.Context())
			cmd.ErrCheck(err)
			if len(res.Addresses) == 0 {
				cmd.ErrCheck(fmt.Errorf("no addrs returned from powergate"))
			}
			fundingAddr = res.Addresses[0]
		}

		conf := service.Config{
			Listener:          listener,
			RewardStore:       rs,
			ClaimStore:        cs,
			AccountStore:      as,
			Sendfil:           sc,
			Analytics:         ac,
			Powergate:         pc.Wallet,
			FundingAddr:       fundingAddr,
			BaseNanoFILReward: baseFilReward,
			Debug:             debug,
		}
		api, err := service.New(conf)
		cmd.ErrCheck(err)

		fmt.Println("Welcome to Hub Filrewards!")

		cmd.HandleInterrupt(func() {
			cmd.ErrCheck(api.Close())
			cmd.ErrCheck(mongoClient.Disconnect(c.Context()))
		})
	},
}
