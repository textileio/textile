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
	AccountKeyCreated
	UserGroupKeyCreated
	MailboxCreated
	OrgCreated
	OrgLeave
	OrgRemoved
	InviteCreated
	GracePeriodStart
	GracePeriodEnd
	BillingSetup
	AccountDestroyed
)

func (e Event) String() string {
	switch e {
	case SignIn:
		return "signin"
	case AccountKeyCreated:
		return "account_key_created"
	case UserGroupKeyCreated:
		return "user_group_key_created"
	case MailboxCreated:
		return "mailbox_created"
	case OrgCreated:
		return "org_created"
	case OrgLeave:
		return "org_leave"
	case OrgRemoved:
		return "org_removed"
	case GracePeriodStart:
		return "grace_period_start"
	case GracePeriodEnd:
		return "grace_period_end"
	case BillingSetup:
		return "billing_setup"
	case AccountDestroyed:
		return "account_destroyed"
	default:
		return fmt.Sprintf("%d", int(e))
	}
}

// GetCorrespondingTrait returns nil or a bool trait that can be set on the profile
func (e Event) GetCorrespondingTrait() *Trait {
	switch e {
	case AccountDestroyed, AccountKeyCreated, BillingSetup, UserGroupKeyCreated, MailboxCreated, OrgCreated, OrgRemoved:
		return &Trait{
			name:  e.String(),
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
