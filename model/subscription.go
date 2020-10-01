package model

import (
	"strconv"
	"time"
)

// AlertName ...
type AlertName struct {
	AlertName string `json:"alert_name"`
}

// Subscription ...
type Subscription struct {
	ID             uint       `gorm:"primary_key" json:"id"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at"`
	CancelledAt    time.Time  `json:"cancelled_at"`
	SubscriptionID int        `json:"subscription_id"`
	PlanID         int        `json:"plan_id"`
	UserID         int        `json:"user_id"`
	Status         string     `json:"status"`
	NextBillDate   time.Time  `json:"next_bill_date"`
	UpdateURL      string     `json:"update_url"`
	CancelURL      string     `json:"cancel_url"`
}

//SubscriptionCreated type
type SubscriptionCreated struct {
	AlertID             string `json:"alert_id"`
	AlertName           string `json:"alert_name"`
	CancelURL           string `json:"cancel_url"`
	CheckoutID          string `json:"checkout_id"`
	Currency            string `json:"currency"`
	Email               string `json:"email"`
	EventTime           string `json:"event_time"`
	LinkedSubscriptions string `json:"linked_subscriptions"`
	MarketingConsent    string `json:"marketing_consent"`
	NextBillDate        string `json:"next_bill_date"`
	Passthrough         string `json:"passthrough"`
	Quantity            string `json:"quantity"`
	Source              string `json:"source"`
	Status              string `json:"status"`
	SubscriptionID      string `json:"subscription_id"`
	SubscriptionPlanID  string `json:"subscription_plan_id"`
	UnitPrice           string `json:"unit_price"`
	UpdateURL           string `json:"update_url"`
	UserID              string `json:"user_id"`
	PSignature          string `json:"p_signature"`
}

// FromCreToSub converts SubscriptionCreated type to Subscription type
func FromCreToSub(subscriptionCreated *SubscriptionCreated) *Subscription {
	subID, _ := strconv.Atoi(subscriptionCreated.SubscriptionID)
	planID, _ := strconv.Atoi(subscriptionCreated.SubscriptionPlanID)
	userID, _ := strconv.Atoi(subscriptionCreated.UserID)

	nextBillDate, _ := time.Parse("2006-01-02", subscriptionCreated.NextBillDate)

	return &Subscription{
		SubscriptionID: subID,
		PlanID:         planID,
		UserID:         userID,
		Status:         subscriptionCreated.Status,
		NextBillDate:   nextBillDate,
		UpdateURL:      subscriptionCreated.UpdateURL,
		CancelURL:      subscriptionCreated.CancelURL,
	}
}

//SubscriptionDTO DTO object for Subscription type
type SubscriptionDTO struct {
	ID             uint      `gorm:"primary_key" json:"id"`
	CancelledAt    time.Time `json:"cancelled_at"`
	SubscriptionID int       `json:"subscription_id"`
	PlanID         int       `json:"plan_id"`
	UserID         int       `json:"user_id"`
	Status         string    `json:"status"`
	NextBillDate   time.Time `json:"next_bill_date"`
	UpdateURL      string    `json:"update_url"`
	CancelURL      string    `json:"cancel_url"`
}

// ToSubscription ...
func ToSubscription(subscriptionDTO *SubscriptionDTO) *Subscription {
	return &Subscription{
		ID:             subscriptionDTO.ID,
		CancelledAt:    subscriptionDTO.CancelledAt,
		SubscriptionID: subscriptionDTO.SubscriptionID,
		PlanID:         subscriptionDTO.PlanID,
		UserID:         subscriptionDTO.UserID,
		Status:         subscriptionDTO.Status,
		NextBillDate:   subscriptionDTO.NextBillDate,
		UpdateURL:      subscriptionDTO.UpdateURL,
		CancelURL:      subscriptionDTO.CancelURL,
	}
}

// ToSubscriptionDTO ...
func ToSubscriptionDTO(subscription *Subscription) *SubscriptionDTO {
	return &SubscriptionDTO{
		ID:             subscription.ID,
		CancelledAt:    subscription.CancelledAt,
		SubscriptionID: subscription.SubscriptionID,
		PlanID:         subscription.PlanID,
		UserID:         subscription.UserID,
		Status:         subscription.Status,
		NextBillDate:   subscription.NextBillDate,
		UpdateURL:      subscription.UpdateURL,
		CancelURL:      subscription.CancelURL,
	}
}

/*
// CREATED
{
    "alert_id": "1033789248",
    "alert_name": "subscription_created",
    "cancel_url": "https://checkout.paddle.com/subscription/cancel?user=6&subscription=3&hash=39dd27dd11467b5e75e99ac4f7ebba854013a71e",
    "checkout_id": "6-11da64a5a5f11c5-08bba2c0cf",
    "currency": "EUR",
    "email": "janet92@example.com",
    "event_time": "2020-09-29 19:08:04",
    "linked_subscriptions": "8, 4, 5",
    "marketing_consent": "1",
    "next_bill_date": "2020-10-22",
    "passthrough": "Example String",
    "quantity": "12",
    "source": "Trial",
    "status": "trialing",
    "subscription_id": "4",
    "subscription_plan_id": "2",
    "unit_price": "unit_price",
    "update_url": "https://checkout.paddle.com/subscription/update?user=5&subscription=2&hash=e8c075ecd29e7da0c31f306986214db7d20bf04d",
    "user_id": "7",
    "p_signature": "Zjc7d7idSudogSLfyYh2r4vYeViFimrfL/ouV0bsQUiPPIMH2e7bPkMGdtYbSboES0GX+/BsouejnbmMuNZttAsSpK9X/hYSXiURQY5qVbvXXu0+cOSatomxwjjI6aYW3N2aKX8I1jDOQf80uLliWfzCCC19ExZjOCS78m6SJWp8lSkEjNpCIFPA82OGwLrVNiPm8JioGOFADCm5V9LYK9WXPw4GuQKiN4HqLnBnwBotmxO79x5/wugy8kvOTwHKJxlBAzi5108j73D5/xWY21Y1z7Vy4g54zpI4BT/Op/REhl3prO9cD+cEky91J8LXmNFXaLw9k6D/GPtQJWJIwckG1veLLTRx5WVuhBCKc7WZ0VRm6GzO3a7ZXv1dom025hdYmsGR928CQ0rv5M0chA0qtw8+xloGF0zI/6YDEvMSLGFVHStkmVwLGwERNFheJK2cIE0MUZdWgh8/8GUU3FZ4UZRQXFX5ofjuy8/nR59OJ1FcQ/OhS04c4ZR7s5dQOS+pSUHQrj4ii8bA2Xw6t9XiKXSldMq6yRKc6+923CtmmxTMVdp1uMUKTdZAmoabZ0IiowhZcXPvBpVPv5zTSbEW8u7q9qgUklaDkydA89Mb4V1QebIaMOrbLII3kk1qo4IJQsxLMgM9Z7cz0unsTkn59sAzeLN0kgmYa+dJcj4="
  }

  // UPDATED
  {
    "alert_id": "285728369",
    "alert_name": "subscription_updated",
    "cancel_url": "https://checkout.paddle.com/subscription/cancel?user=4&subscription=7&hash=bd02863f35a474d640cdf370a72d4ee66ba5afeb",
    "checkout_id": "6-c5a7d401ba4e852-eea9686a7c",
    "currency": "USD",
    "email": "lue.schimmel@example.org",
    "event_time": "2020-09-30 06:31:37",
    "linked_subscriptions": "5, 3, 6",
    "marketing_consent": "1",
    "new_price": "new_price",
    "new_quantity": "new_quantity",
    "new_unit_price": "new_unit_price",
    "next_bill_date": "2020-10-12",
    "old_next_bill_date": "old_next_bill_date",
    "old_price": "old_price",
    "old_quantity": "old_quantity",
    "old_status": "old_status",
    "old_subscription_plan_id": "old_subscription_plan_id",
    "old_unit_price": "old_unit_price",
    "passthrough": "Example String",
    "status": "trialing",
    "subscription_id": "6",
    "subscription_plan_id": "1",
    "update_url": "https://checkout.paddle.com/subscription/update?user=6&subscription=7&hash=6818498ceed0dc9bc47057576707574ab5bc5d3b",
    "user_id": "9",
    "p_signature": "xQVGXPrrpT8yZ0bb7clxKQHnj4pjEvAQHK+V2uSdXuAiUrZMHqG2vLpW7/U0zNBtWp+Gtk+l0LXdLEUKKACZ+TZnUEDqmGB6hIPrp2mA4kk+y8jjwTTGYS9n7qMdxHXxIsVxXAMwyHdZMz8YvT07+DNMAQ/iqQW94xvCNxYBhqah1ypQf2aeSXs5bCwqvz3MZ+g3cdng4XXUQqloZeDFFKnt2wu2v6rwiOdTnou3BnUgXwjELoxi6ybr3GiAdpFWpkb5SnSD0ohcKvrswBMF1WGaej4LZ8Qc7Bg1i/4KMgl0hyS7wUa9wgvFOj+UI+BdZT4pUw+zopDbC+CLEz6JwkeyifUpD66lHtCD9zJpJ6ItQKP/2pPgRsZm8EHdEPzNhfJZHdgDH9XLocNMdTrWji8hkhG5up9L2Jm4fFCE6cw3D+Prbiz12llYHDyivwX4xT5f0hh4K0s6+789JI1KGVis5u+isYBPdIN39tBJIClxOy0KGCfWA+Dvsm+OJInn8C1EhbPak6GkpQKPXy310qub1piNLg+2BcuzNLgy9+t6fLk4E5tqiZO5VcVIMinJZZ2whP0yCy16UDQQsK2oBU1UzupRy4HSeuvAtcKb/Nkwop4oX/XCNYGlEoiHmNnr5/4dXC5MGSttt1+ZdEycBtEFEA4YF6KGVuIHxeHADfw="
  }
*/
