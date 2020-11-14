package common

import (
	"errors"
	"fmt"

	stripe "github.com/stripe/stripe-go/v72"
)

var (
	// ErrExceedsFreeQuota indicates the requested operation exceeds the free quota.
	ErrExceedsFreeQuota = errors.New("request exceeds free quota")

	// ErrSubscriptionExists indicates the subscription already exists and has a healthy status.
	ErrSubscriptionExists = errors.New("subscription exists")

	// ErrSubscriptionCanceled indicates the subscription was canceled by the user or as a result of failed payment.
	ErrSubscriptionCanceled = errors.New("subscription canceled")

	// ErrSubscriptionPaymentRequired indicates the subscription is in a terminal status as a result of failed payment.
	ErrSubscriptionPaymentRequired = errors.New("subscription payment required")
)

// StatusCheck returns a non-nil error if the subscription status is considered healthy.
func StatusCheck(status string) error {
	switch stripe.SubscriptionStatus(status) {
	case stripe.SubscriptionStatusActive,
		stripe.SubscriptionStatusIncomplete:
		return nil
	case stripe.SubscriptionStatusCanceled:
		return ErrSubscriptionCanceled
	case stripe.SubscriptionStatusIncompleteExpired,
		stripe.SubscriptionStatusPastDue,
		stripe.SubscriptionStatusUnpaid:
		return ErrSubscriptionPaymentRequired
	default:
		return fmt.Errorf("unhandled subscription status: %s", status)
	}
}
