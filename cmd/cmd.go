package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/spf13/viper"
	tc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	bc "github.com/textileio/textile/v2/api/bucketsd/client"
	"github.com/textileio/textile/v2/api/common"
	fc "github.com/textileio/textile/v2/api/filecoin/client"
	hc "github.com/textileio/textile/v2/api/hubd/client"
	mi "github.com/textileio/textile/v2/api/mindexd/client"
	uc "github.com/textileio/textile/v2/api/usersd/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	// Timeout is the default timeout used for most commands.
	Timeout = time.Minute * 10
	// PushTimeout is the command timeout used when pushing bucket changes.
	PushTimeout = time.Hour * 24
	// PullTimeout is the command timeout used when pulling bucket changes.
	PullTimeout = time.Hour * 24
	// ArchiveWatchTimeout is the command timeout used when watching archive status messages.
	ArchiveWatchTimeout = time.Hour * 12

	// Bold is a styler used to make the output text bold.
	Bold = promptui.Styler(promptui.FGBold)

	// Repo organization/repo where client releases are published
	Repo = "textileio/textile"
)

// Flag describes a command flag.
type Flag struct {
	Key      string
	DefValue interface{}
}

// Config describes a command config params and file info.
type Config struct {
	Viper  *viper.Viper
	File   string
	Dir    string
	Name   string
	Flags  map[string]Flag
	EnvPre string
	Global bool
}

// ConfConfig is used to generate new messages configs.
type ConfConfig struct {
	Dir       string // Config directory base name
	Name      string // Name of the mailbox config file
	Type      string // Type is the type of config file (yaml/json)
	EnvPrefix string // A prefix that will be expected on env vars
}

// NewConfig uses values from ConfConfig to contruct a new config.
func (cc ConfConfig) NewConfig(pth string, flags map[string]Flag, global bool) (c *Config, fileExists bool, err error) {
	v := viper.New()
	v.SetConfigType(cc.Type)
	c = &Config{
		Viper:  v,
		Dir:    cc.Dir,
		Name:   cc.Name,
		Flags:  flags,
		EnvPre: cc.EnvPrefix,
		Global: global,
	}
	fileExists = FindConfigFile(c, pth)
	return c, fileExists, nil
}

// Clients wraps all the possible hubd/buckd clients.
type Clients struct {
	Buckets    *bc.Client
	Threads    *tc.Client
	Hub        *hc.Client
	Users      *uc.Client
	Filecoin   *fc.Client
	MinerIndex *mi.Client
}

// NewClients returns a new clients object pointing to the target address.
// If isHub is true, the hub's admin and user clients are also created.
func NewClients(hubTarget string, isHub bool, minerIndexTarget string) *Clients {
	var hubOpts []grpc.DialOption
	auth := common.Credentials{}
	if strings.Contains(hubTarget, "443") {
		creds := credentials.NewTLS(&tls.Config{})
		hubOpts = append(hubOpts, grpc.WithTransportCredentials(creds))
		auth.Secure = true
	} else {
		hubOpts = append(hubOpts, grpc.WithInsecure())
	}
	hubOpts = append(hubOpts, grpc.WithPerRPCCredentials(auth))

	c := &Clients{}
	var err error
	c.Threads, err = tc.NewClient(hubTarget, hubOpts...)
	if err != nil {
		Fatal(err)
	}
	c.Buckets, err = bc.NewClient(hubTarget, hubOpts...)
	if err != nil {
		Fatal(err)
	}
	if isHub {
		c.Hub, err = hc.NewClient(hubTarget, hubOpts...)
		if err != nil {
			Fatal(err)
		}
		c.Users, err = uc.NewClient(hubTarget, hubOpts...)
		if err != nil {
			Fatal(err)
		}

		var minerIndexOpts []grpc.DialOption
		if strings.Contains(minerIndexTarget, "443") {
			creds := credentials.NewTLS(&tls.Config{})
			minerIndexOpts = append(minerIndexOpts, grpc.WithTransportCredentials(creds))
		} else {
			minerIndexOpts = append(minerIndexOpts, grpc.WithInsecure())
		}

		c.MinerIndex, err = mi.NewClient(minerIndexTarget, minerIndexOpts...)
		if err != nil {
			Fatal(err)
		}
	}
	c.Filecoin, err = fc.NewClient(hubTarget, hubOpts...)
	if err != nil {
		Fatal(err)
	}
	return c
}

// Close closes all the clients.
func (c *Clients) Close() {
	if c.Threads != nil {
		if err := c.Threads.Close(); err != nil {
			Fatal(err)
		}
	}
	if c.Buckets != nil {
		if err := c.Buckets.Close(); err != nil {
			Fatal(err)
		}
	}
	if c.Hub != nil {
		if err := c.Hub.Close(); err != nil {
			Fatal(err)
		}
	}
	if c.Users != nil {
		if err := c.Users.Close(); err != nil {
			Fatal(err)
		}
	}
}

// Thread wraps details about a thread.
type Thread struct {
	ID    thread.ID `json:"id"`
	Label string    `json:"label"`
	Name  string    `json:"name"`
	Type  string    `json:"type"`
}

// ListThreads returns a list of threads for the context.
// In a hub context, this will only list threads that the context
// has access to.
// dbsOnly filters threads that do not belong to a database.
func (c *Clients) ListThreads(ctx context.Context, dbsOnly bool) []Thread {
	var threads []Thread
	if c.Users != nil {
		list, err := c.Users.ListThreads(ctx)
		if err != nil {
			Fatal(err)
		}
		for _, t := range list.List {
			if dbsOnly && !t.IsDb {
				continue
			}
			id, err := thread.Cast(t.Id)
			if err != nil {
				Fatal(err)
			}
			if t.Name == "" {
				t.Name = "unnamed"
			}
			threads = append(threads, Thread{
				ID:    id,
				Label: id.String(),
				Name:  t.Name,
				Type:  GetThreadType(t.IsDb),
			})
		}
	} else {
		list, err := c.Threads.ListDBs(ctx)
		if err != nil {
			Fatal(err)
		}
		for id, t := range list {
			if t.Name == "" {
				t.Name = "unnamed"
			}
			threads = append(threads, Thread{
				ID:    id,
				Label: id.String(),
				Name:  t.Name,
				Type:  "db",
			})
		}
	}
	return threads
}

// GetThreadType returns a string representation of the type of a thread.
func GetThreadType(isDB bool) string {
	if isDB {
		return "db"
	} else {
		return "log"
	}
}

// SelectThread presents the caller with a choice of threads.
func (c *Clients) SelectThread(ctx context.Context, label, successMsg string, dbsOnly bool) Thread {
	threads := c.ListThreads(ctx, dbsOnly)
	var name string
	if len(threads) == 0 {
		name = "default"
	}
	threads = append(threads, Thread{Label: "Create new", Name: name, Type: "db"})
	prompt := promptui.Select{
		Label: label,
		Items: threads,
		Templates: &promptui.SelectTemplates{
			Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ .Label | bold }} {{ .Name | faint | bold }}`, promptui.IconSelect),
			Inactive: `{{ .Label | faint }} {{ .Name | faint | bold }}`,
			Details:  `{{ "(Type:" | faint }} {{ .Type | faint }}{{ ")" | faint }}`,
			Selected: successMsg,
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		End("")
	}
	return threads[index]
}
