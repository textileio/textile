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
	"github.com/textileio/powergate/exe/cli/cmd"
	bc "github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/api/common"
	hc "github.com/textileio/textile/api/hub/client"
	uc "github.com/textileio/textile/api/users/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	Timeout = time.Minute
	Bold    = promptui.Styler(promptui.FGBold)
)

type Flag struct {
	Key      string
	DefValue interface{}
}

type Config struct {
	Viper  *viper.Viper
	File   string
	Dir    string
	Name   string
	Flags  map[string]Flag
	EnvPre string
	Global bool
}

type ClientsCtx interface {
	Auth(time.Duration) (context.Context, context.CancelFunc)
	Thread(time.Duration) (context.Context, context.CancelFunc)
}

type Clients struct {
	Buckets *bc.Client
	Threads *tc.Client
	Hub     *hc.Client
	Users   *uc.Client

	Ctx ClientsCtx
}

func NewClients(target string, hub bool, ctx ClientsCtx) *Clients {
	var opts []grpc.DialOption
	auth := common.Credentials{}
	if strings.Contains(target, "443") {
		creds := credentials.NewTLS(&tls.Config{})
		opts = append(opts, grpc.WithTransportCredentials(creds))
		auth.Secure = true
	} else {
		opts = append(opts, grpc.WithInsecure())
	}
	opts = append(opts, grpc.WithPerRPCCredentials(auth))

	c := &Clients{Ctx: ctx}
	var err error
	c.Threads, err = tc.NewClient(target, opts...)
	if err != nil {
		cmd.Fatal(err)
	}
	c.Buckets, err = bc.NewClient(target, opts...)
	if err != nil {
		cmd.Fatal(err)
	}
	if hub {
		c.Hub, err = hc.NewClient(target, opts...)
		if err != nil {
			cmd.Fatal(err)
		}
		c.Users, err = uc.NewClient(target, opts...)
		if err != nil {
			cmd.Fatal(err)
		}
	}
	return c
}

func (c *Clients) Close() {
	if c.Threads != nil {
		if err := c.Threads.Close(); err != nil {
			cmd.Fatal(err)
		}
	}
	if c.Buckets != nil {
		if err := c.Buckets.Close(); err != nil {
			cmd.Fatal(err)
		}
	}
	if c.Hub != nil {
		if err := c.Hub.Close(); err != nil {
			cmd.Fatal(err)
		}
	}
	if c.Users != nil {
		if err := c.Users.Close(); err != nil {
			cmd.Fatal(err)
		}
	}
}

type ThreadItem struct {
	ID   string
	Name string
	Type string
}

func (c *Clients) SelectThread(label, successMsg string, dbsOnly bool) *ThreadItem {
	if c.Users == nil {
		// @todo: List all threadsd threads
	}
	ctx, cancel := c.Ctx.Auth(Timeout)
	defer cancel()
	list, err := c.Users.ListThreads(ctx)
	if err != nil {
		cmd.Fatal(err)
	}

	var items []*ThreadItem
	for _, t := range list.List {
		if dbsOnly && !t.IsDB {
			continue
		}
		id, err := thread.Cast(t.ID)
		if err != nil {
			cmd.Fatal(err)
		}
		if t.Name == "" {
			t.Name = "unnamed"
		}
		items = append(items, &ThreadItem{
			ID:   id.String(),
			Name: t.Name,
			Type: GetThreadType(t.IsDB),
		})
	}
	var name string
	if len(items) == 0 {
		name = "default"
	}
	items = append(items, &ThreadItem{ID: "Create new", Name: name, Type: "db"})

	prompt := promptui.Select{
		Label: label,
		Items: items,
		Templates: &promptui.SelectTemplates{
			Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ .ID | bold }} {{ .Name | faint | bold }}`, promptui.IconSelect),
			Inactive: `{{ .ID | faint }} {{ .Name | faint | bold }}`,
			Details:  `{{ "(Type:" | faint }} {{ .Type | faint }}{{ ")" | faint }}`,
			Selected: successMsg,
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		End("")
	}
	return items[index]
}

func GetThreadType(isDB bool) string {
	if isDB {
		return "db"
	} else {
		return "log"
	}
}
