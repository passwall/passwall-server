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

// ToSubscription ...
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
*/
