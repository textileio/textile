package buckets

import (
	"context"
	"errors"
	"fmt"
	"github.com/multiformats/go-multibase"
	"strings"

	"github.com/textileio/go-threads/core/thread"
	pb "github.com/textileio/textile/v2/api/bucketsd/pb"
)

const (
	// CollectionName is the name of the threaddb collection used for buckets.
	CollectionName = "buckets"
	// SeedName is the file name reserved for a random bucket seed.
	SeedName = ".textileseed"
)

var (
	// ErrNonFastForward is returned when an update in non-fast-forward.
	ErrNonFastForward = fmt.Errorf("update is non-fast-forward")

	// ErrNoCurrentArchive is returned when not status about the last archive
	// can be retrieved, since the bucket was never archived.
	ErrNoCurrentArchive = fmt.Errorf("the bucket was never archived")

	// ErrZeroBalance is returned when archiving a bucket which
	// underlying Account Powergate user balance is zero.
	ErrZeroBalance = errors.New("wallet FIL balance is zero, if recently created wait 30s")
)

type ctxKey string

// BucketOwner provides owner context to the bucket service.
type BucketOwner struct {
	StorageUsed      int64
	StorageAvailable int64
	StorageDelta     int64
}

func NewBucketOwnerContext(ctx context.Context, owner *BucketOwner) context.Context {
	return context.WithValue(ctx, ctxKey("bucketOwner"), owner)
}

func BucketOwnerFromContext(ctx context.Context) (*BucketOwner, bool) {
	owner, ok := ctx.Value(ctxKey("bucketOwner")).(*BucketOwner)
	return owner, ok
}

// Role describes an access role for a bucket item.
type Role int

const (
	None Role = iota
	Reader
	Writer
	Admin
)

// NewRoleFromString returns the role associated with the given string.
func NewRoleFromString(s string) (Role, error) {
	switch strings.ToLower(s) {
	case "none":
		return None, nil
	case "reader":
		return Reader, nil
	case "writer":
		return Writer, nil
	case "admin":
		return Admin, nil
	default:
		return None, fmt.Errorf("invalid role: %s", s)
	}
}

// String returns the string representation of the role.
func (r Role) String() string {
	switch r {
	case None:
		return "None"
	case Reader:
		return "Reader"
	case Writer:
		return "Writer"
	case Admin:
		return "Admin"
	default:
		return "Invalid"
	}
}

// RoleToPb maps native type roles to protobuf type roles.
func RoleToPb(role Role) (pb.PathAccessRole, error) {
	var pr pb.PathAccessRole
	switch role {
	case None:
		pr = pb.PathAccessRole_PATH_ACCESS_ROLE_UNSPECIFIED
	case Reader:
		pr = pb.PathAccessRole_PATH_ACCESS_ROLE_READER
	case Writer:
		pr = pb.PathAccessRole_PATH_ACCESS_ROLE_WRITER
	case Admin:
		pr = pb.PathAccessRole_PATH_ACCESS_ROLE_ADMIN
	default:
		return pb.PathAccessRole_PATH_ACCESS_ROLE_UNSPECIFIED, fmt.Errorf("unknown path access role %d", role)
	}

	return pr, nil
}

// RolesToPb maps native type roles to protobuf type roles.
func RolesToPb(in map[string]Role) (map[string]pb.PathAccessRole, error) {
	roles := make(map[string]pb.PathAccessRole)
	for k, r := range in {
		var pr pb.PathAccessRole
		pr, err := RoleToPb(r)
		if err != nil {
			return nil, err
		}
		roles[k] = pr
	}
	return roles, nil
}

// RoleFromPb maps protobuf type role to native type role.
func RoleFromPb(pr pb.PathAccessRole) (Role, error) {
	var r Role
	switch pr {
	case pb.PathAccessRole_PATH_ACCESS_ROLE_UNSPECIFIED:
		r = None
	case pb.PathAccessRole_PATH_ACCESS_ROLE_READER:
		r = Reader
	case pb.PathAccessRole_PATH_ACCESS_ROLE_WRITER:
		r = Writer
	case pb.PathAccessRole_PATH_ACCESS_ROLE_ADMIN:
		r = Admin
	default:
		return None, fmt.Errorf("unknown path access role %d", pr)
	}
	return r, nil
}

// RolesFromPb maps protobuf type roles to native type roles.
func RolesFromPb(in map[string]pb.PathAccessRole) (map[string]Role, error) {
	roles := make(map[string]Role)
	for k, pr := range in {
		if err := ValidateAccessRoleKey(k); err != nil {
			return nil, err
		}
		var r Role
		r, err := RoleFromPb(pr)
		if err != nil {
			return nil, err
		}
		roles[k] = r
	}
	return roles, nil
}

// validate key
func ValidateAccessRoleKey(k string) error {
	if k == "*" {
		return nil
	}

	// check if a valid pub key
	pk := &thread.Libp2pPubKey{}
	if err := pk.UnmarshalString(k); err != nil {
		// check if a valid token hash
		if _, bytes, err2 := multibase.Decode(k); err2 != nil || len(bytes) != 32 {
			return fmt.Errorf("unmarshaling role public key: %s", err)
		}
	}
	return nil
}
