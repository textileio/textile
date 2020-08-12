package local

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/buckets"
)

// GetPathAccessRoles returns access roles for a path.
func (b *Bucket) GetPathAccessRoles(ctx context.Context, pth string) (roles map[string]buckets.Role, err error) {
	ctx, err = b.context(ctx)
	if err != nil {
		return
	}
	return b.clients.Buckets.GetPathAccessRoles(ctx, b.Key(), pth)
}

// EditPathAccessRoles updates path access roles by merging the given roles with existing roles
// and returns the merged roles.
// roles is a map of string marshaled public keys to path roles. A non-nil error is returned
// if the map keys are not unmarshalable to public keys.
// To delete a role for a public key, set its value to buckets.None.
func (b *Bucket) EditPathAccessRoles(ctx context.Context, pth string, roles map[string]buckets.Role) (merged map[string]buckets.Role, err error) {
	ctx, err = b.context(ctx)
	if err != nil {
		return
	}
	err = b.clients.Buckets.EditPathAccessRoles(ctx, b.Key(), pth, roles)
	if err != nil {
		return
	}
	return b.clients.Buckets.GetPathAccessRoles(ctx, b.Key(), pth)
}

// PathInvite wraps information needed to collaborate on a bucket path.
type PathInvite struct {
	Key  string `json:"key"`
	Path string `json:"path"`
}

// SendPathInvite sends a message containing a bucket key and path,
// which can be used to access a shared file / folder.
func (b *Bucket) SendPathInvite(ctx context.Context, from thread.Identity, to thread.PubKey, pth string) error {
	if b.clients.Users == nil {
		return fmt.Errorf("hub is required to send invites")
	}
	msg, err := json.Marshal(&PathInvite{
		Key:  b.Key(),
		Path: pth,
	})
	if err != nil {
		return err
	}
	_, err = b.clients.Users.SendMessage(ctx, from, to, msg)
	return err
}
