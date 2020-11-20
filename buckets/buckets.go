package buckets

import (
	"context"
	"errors"
	"fmt"
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

// RolesToPb maps native type roles to protobuf type roles.
func RolesToPb(in map[string]Role) (map[string]pb.PathAccessRole, error) {
	roles := make(map[string]pb.PathAccessRole)
	for k, r := range in {
		var pr pb.PathAccessRole
		switch r {
		case None:
			pr = pb.PathAccessRole_PATH_ACCESS_ROLE_UNSPECIFIED
		case Reader:
			pr = pb.PathAccessRole_PATH_ACCESS_ROLE_READER
		case Writer:
			pr = pb.PathAccessRole_PATH_ACCESS_ROLE_WRITER
		case Admin:
			pr = pb.PathAccessRole_PATH_ACCESS_ROLE_ADMIN
		default:
			return nil, fmt.Errorf("unknown path access role %d", r)
		}
		roles[k] = pr
	}
	return roles, nil
}

// RolesFromPb maps protobuf type roles to native type roles.
func RolesFromPb(in map[string]pb.PathAccessRole) (map[string]Role, error) {
	roles := make(map[string]Role)
	for k, pr := range in {
		if k != "*" {
			pk := &thread.Libp2pPubKey{}
			if err := pk.UnmarshalString(k); err != nil {
				return nil, fmt.Errorf("unmarshaling role public key: %s", err)
			}
		}
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
			return nil, fmt.Errorf("unknown path access role %d", pr)
		}
		roles[k] = r
	}
	return roles, nil
}
