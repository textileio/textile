package analytics

import (
	"fmt"
)

// Trait is a bool trait set on user by specific events
type Trait struct {
	name  string
	value bool
}

// Event is a type of usage event
type Event int

const (
	SignIn Event = iota
	AccountDestroyed
	KeyAccountCreated
	KeyUserGroupCreated
	OrgCreated
	OrgLeave
	OrgDestroyed
	OrgInviteCreated
	GracePeriodStart
	GracePeriodEnd
	BillingSetup
	BucketCreated
	BucketArchiveCreated
	MailboxCreated
	ThreadDbCreated
)

func (e Event) String() string {
	switch e {
	case SignIn:
		return "signin"
	case AccountDestroyed:
		return "account_destroyed"
	case KeyAccountCreated:
		return "account_key_created"
	case KeyUserGroupCreated:
		return "user_group_key_created"
	case OrgCreated:
		return "org_created"
	case OrgLeave:
		return "org_leave"
	case OrgDestroyed:
		return "org_destroyed"
	case OrgInviteCreated:
		return "org_invite_created"
	case GracePeriodStart:
		return "grace_period_start"
	case GracePeriodEnd:
		return "grace_period_end"
	case BillingSetup:
		return "billing_setup"
	case BucketCreated:
		return "bucket_created"
	case BucketArchiveCreated:
		return "bucket_archive_created"
	case MailboxCreated:
		return "mailbox_created"
	case ThreadDbCreated:
		return "threaddb_created"
	default:
		return fmt.Sprintf("%d", int(e))
	}
}

// GetCorrespondingTrait returns nil or a bool trait that should be set on the profile
func (e Event) GetCorrespondingTrait() *Trait {
	switch e {
	case AccountDestroyed:
		return &Trait{
			name:  "destroyed",
			value: true,
		}
	case GracePeriodStart:
		return &Trait{
			name:  "in_grace_period",
			value: true,
		}
	case GracePeriodEnd:
		return &Trait{
			name:  "in_grace_period",
			value: false,
		}
	default:
		return nil
	}
}
