package local

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/v2/cmd"
)

var (
	// ErrNotAMailbox indicates the given path is not within a mailbox.
	ErrNotAMailbox = errors.New("not a mailbox (or any of the parent directories): .textile")
	// ErrMailboxExists is used during initialization to indicate the path already contains a mailbox.
	ErrMailboxExists = errors.New("mailbox is already initialized")
	// ErrIdentityRequired indicates the operation requires a thread Identity but none was given.
	ErrIdentityRequired = errors.New("thread Identity is required")
	// ErrAPIKeyRequired indicates the operation requires am API keybut none was given.
	ErrAPIKeyRequired = errors.New("api key is required")

	flags = map[string]cmd.Flag{
		"identity":   {Key: "identity", DefValue: ""},
		"api_key":    {Key: "api_key", DefValue: ""},
		"api_secret": {Key: "api_secret", DefValue: ""},
	}
)

// DefaultConfConfig returns the default ConfConfig.
func DefaultConfConfig() cmd.ConfConfig {
	return cmd.ConfConfig{
		Dir:       ".textile",
		Name:      "config",
		Type:      "yaml",
		EnvPrefix: "MAIL",
	}
}

// Mail is used to create new mailboxes based on the provided clients and config.
type Mail struct {
	config  cmd.ConfConfig
	clients *cmd.Clients
}

// NewMail creates Mail from clients and config.
func NewMail(clients *cmd.Clients, config cmd.ConfConfig) *Mail {
	return &Mail{clients: clients, config: config}
}

// Clients returns the underlying clients object.
func (m *Mail) Clients() *cmd.Clients {
	return m.clients
}

// Config contains details for a new local mailbox.
type Config struct {
	// Path is the path in which the new mailbox should be created (required).
	Path string
	// Identity is the thread.Identity of the mailbox owner (required).
	// It's value may be inflated from a --identity flag or {EnvPrefix}_IDENTITY env variable.
	Identity thread.Identity
	// APIKey is hub API key (required).
	// It's value may be inflated from a --api-key flag or {EnvPrefix}_API_KEY env variable.
	APIKey string
	// APISecret is hub API key secret (optional).
	// It's value may be inflated from a --api-secret flag or {EnvPrefix}_API_SECRET env variable.
	APISecret string
}

// NewConfigFromCmd returns a config by inflating values from the given cobra command and path.
// First, flags for "identity", "api_key", and "api_secret" are used if they exist.
// If still unset, the env vars {EnvPrefix}_IDENTITY, {EnvPrefix}_API_KEY, and {EnvPrefix}_API_SECRET are used.
func (m *Mail) NewConfigFromCmd(c *cobra.Command, pth string) (conf Config, err error) {
	conf.Path = pth
	id := cmd.GetFlagOrEnvValue(c, "identity", m.config.EnvPrefix)
	if id == "" {
		return conf, ErrIdentityRequired
	}
	idb, err := base64.StdEncoding.DecodeString(id)
	if err != nil {
		return
	}
	conf.Identity = &thread.Libp2pIdentity{}
	if err = conf.Identity.UnmarshalBinary(idb); err != nil {
		return
	}
	conf.APIKey = cmd.GetFlagOrEnvValue(c, "api_key", m.config.EnvPrefix)
	if conf.APIKey == "" {
		return conf, ErrAPIKeyRequired
	}
	conf.APISecret = cmd.GetFlagOrEnvValue(c, "api_secret", m.config.EnvPrefix)
	return conf, nil
}

// NewMailbox initializes a new mailbox from the config.
func (m *Mail) NewMailbox(ctx context.Context, conf Config) (box *Mailbox, err error) {
	// Ensure we're not going to overwrite an existing local config
	cwd, err := filepath.Abs(conf.Path)
	if err != nil {
		return
	}
	mc, found, err := m.config.NewConfig(cwd, flags, true)
	if err != nil {
		return
	}
	if found {
		return nil, ErrMailboxExists
	}

	// Check config values
	if conf.Identity == nil {
		return nil, ErrIdentityRequired
	}
	idb, err := conf.Identity.MarshalBinary()
	if err != nil {
		return
	}
	mc.Viper.Set("identity", base64.StdEncoding.EncodeToString(idb))
	mc.Viper.Set("api_key", conf.APIKey)
	if conf.APIKey == "" {
		return nil, ErrAPIKeyRequired
	}
	mc.Viper.Set("api_secret", conf.APISecret)

	box = &Mailbox{
		cwd:     cwd,
		conf:    mc,
		clients: m.clients,
		id:      conf.Identity,
	}
	ctx, err = box.context(ctx)
	if err != nil {
		return
	}
	if _, err = m.clients.Users.SetupMailbox(ctx); err != nil {
		return
	}

	// Write the local config to disk
	dir := filepath.Join(cwd, box.conf.Dir)
	if err = os.MkdirAll(dir, os.ModePerm); err != nil {
		return
	}
	config := filepath.Join(dir, box.conf.Name+".yml")
	if err = box.conf.Viper.WriteConfigAs(config); err != nil {
		return
	}
	cfile, err := filepath.Abs(config)
	if err != nil {
		return
	}
	box.conf.Viper.SetConfigFile(cfile)

	return box, nil
}

// GetLocalMailbox loads and returns the mailbox at path if it exists.
func (m *Mail) GetLocalMailbox(_ context.Context, pth string) (*Mailbox, error) {
	cwd, err := filepath.Abs(pth)
	if err != nil {
		return nil, err
	}
	mc, found, err := m.config.NewConfig(cwd, flags, true)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNotAMailbox
	}
	cmd.ExpandConfigVars(mc.Viper, mc.Flags)
	box := &Mailbox{
		cwd:     cwd,
		conf:    mc,
		clients: m.clients,
	}
	if err = box.loadIdentity(); err != nil {
		return nil, err
	}
	return box, nil
}
