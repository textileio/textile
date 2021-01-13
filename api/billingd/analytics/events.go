package analytics

import (
	"fmt"
)

// Event is a type of usage event
type Event int

const (
	SignIn Event = iota
	AccountDestroyed
	KeyAccountCreated
	KeyUserCreated
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
		return "key_account_created"
	case KeyUserCreated:
		return "key_user_created"
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
