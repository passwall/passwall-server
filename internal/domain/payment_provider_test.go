package domain

import (
	"testing"
	"time"
)

func TestAdminListProviderDistinguishesFreeFromManual(t *testing.T) {
	free := &Subscription{State: SubStateActive, Plan: &Plan{Code: "free-monthly"}}
	if got := AdminListProvider(free); got != PaymentProviderNone {
		t.Fatalf("free provider = %s, want none", got)
	}

	end := time.Now().Add(24 * time.Hour)
	manual := &Subscription{State: SubStateActive, RenewAt: &end, Plan: &Plan{Code: "pro-yearly", PriceCents: 1900}}
	if got := AdminListProvider(manual); got != PaymentProviderManual {
		t.Fatalf("manual provider = %s, want manual", got)
	}

	stripeID := "sub_123"
	stripe := &Subscription{State: SubStateActive, StripeSubscriptionID: &stripeID, Plan: &Plan{Code: "pro-yearly", PriceCents: 1900}}
	if got := AdminListProvider(stripe); got != PaymentProviderStripe {
		t.Fatalf("stripe provider = %s, want stripe", got)
	}

	if AdminListProvider(nil) != PaymentProviderNone {
		t.Fatal("nil subscription should be none")
	}
}
