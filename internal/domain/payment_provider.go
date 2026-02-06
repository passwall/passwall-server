package domain

import "strings"

// PaymentProvider represents the payment provider for a subscription
type PaymentProvider string

const (
	PaymentProviderStripe     PaymentProvider = "stripe"
	PaymentProviderRevenueCat PaymentProvider = "revenuecat"
	PaymentProviderManual     PaymentProvider = "manual"
	PaymentProviderNone       PaymentProvider = "none"
)

func (p PaymentProvider) String() string { return string(p) }

// IsRevenueCat returns true if the provider is RevenueCat (mobile in-app purchase)
func (p PaymentProvider) IsRevenueCat() bool {
	return p == PaymentProviderRevenueCat
}

// IsStripe returns true if the provider is Stripe (web payment)
func (p PaymentProvider) IsStripe() bool {
	return p == PaymentProviderStripe
}

// IsManagedExternally returns true if the subscription is managed by an external store
// (App Store / Play Store via RevenueCat). These subscriptions can only be
// canceled/modified from the respective store — not from our API.
func (p PaymentProvider) IsManagedExternally() bool {
	return p == PaymentProviderRevenueCat
}

// DetectPaymentProvider determines the payment provider from a subscription ID.
// RevenueCat subscriptions use the format "rc_{store}_{transaction_id}".
// Empty or nil subscription IDs indicate a manual/admin grant.
func DetectPaymentProvider(subscriptionID *string) PaymentProvider {
	if subscriptionID == nil || *subscriptionID == "" {
		return PaymentProviderManual
	}
	if strings.HasPrefix(*subscriptionID, "rc_") {
		return PaymentProviderRevenueCat
	}
	return PaymentProviderStripe
}

// DetectStoreFromSubscriptionID extracts the store name from a RevenueCat subscription ID.
// Returns empty string for non-RevenueCat subscriptions.
// RevenueCat format: "rc_{STORE}_{transaction_id}" e.g. "rc_PLAY_STORE_abc123"
func DetectStoreFromSubscriptionID(subscriptionID *string) string {
	if subscriptionID == nil || !strings.HasPrefix(*subscriptionID, "rc_") {
		return ""
	}

	// Remove "rc_" prefix
	rest := strings.TrimPrefix(*subscriptionID, "rc_")

	// Known store prefixes (from RevenueCat)
	stores := []string{
		"APP_STORE",
		"PLAY_STORE",
		"MAC_APP_STORE",
		"AMAZON",
		"PROMOTIONAL",
		"STRIPE",
	}

	for _, store := range stores {
		if strings.HasPrefix(rest, store+"_") || rest == store {
			return store
		}
	}

	// Fallback: take everything before the last underscore as store
	if idx := strings.Index(rest, "_"); idx > 0 {
		return rest[:idx]
	}
	return rest
}

// StoreDisplayName returns a user-friendly name for the store
func StoreDisplayName(store string) string {
	switch store {
	case "APP_STORE":
		return "Apple App Store"
	case "MAC_APP_STORE":
		return "Mac App Store"
	case "PLAY_STORE":
		return "Google Play Store"
	case "AMAZON":
		return "Amazon Appstore"
	case "PROMOTIONAL":
		return "Promotional"
	case "STRIPE":
		return "Stripe"
	default:
		return store
	}
}
