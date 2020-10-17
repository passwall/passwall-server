package model

import (
	"net/http"
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
	Email          string     `json:"email"`
	Status         string     `json:"status"`
	NextBillDate   time.Time  `json:"next_bill_date"`
	UpdateURL      string     `json:"update_url"`
	CancelURL      string     `json:"cancel_url"`
}

type SubscriptionHook struct {
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
func RequestToSub(r *http.Request) *Subscription {
	subID, _ := strconv.Atoi(r.FormValue("subscription_id"))
	planID, _ := strconv.Atoi(r.FormValue("subscription_plan_id"))
	userID, _ := strconv.Atoi(r.FormValue("user_id"))

	status := r.FormValue("status")
	if r.FormValue("status") == "trialing" {
		status = "active"
	}

	nextBillDate, _ := time.Parse("2006-01-02", r.FormValue("next_bill_date"))

	return &Subscription{
		SubscriptionID: subID,
		PlanID:         planID,
		UserID:         userID,
		Email:          r.FormValue("email"),
		Status:         status,
		NextBillDate:   nextBillDate,
		UpdateURL:      r.FormValue("update_url"),
		CancelURL:      r.FormValue("cancel_url"),
	}
}

//SubscriptionDTO DTO object for Subscription type
type SubscriptionDTO struct {
	ID             uint      `gorm:"primary_key" json:"id"`
	CancelledAt    time.Time `json:"cancelled_at"`
	SubscriptionID int       `json:"subscription_id"`
	PlanID         int       `json:"plan_id"`
	UserID         int       `json:"user_id"`
	Email          string    `json:"email"`
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
		Email:          subscriptionDTO.Email,
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
		Email:          subscription.Email,
		Status:         subscription.Status,
		NextBillDate:   subscription.NextBillDate,
		UpdateURL:      subscription.UpdateURL,
		CancelURL:      subscription.CancelURL,
	}
}

/*
// CREATED
{
    "alert_id": "37888873",
    "alert_name": "subscription_created",
    "cancel_url": "https://checkout.paddle.com/subscription/cancel?user=21305808&subscription=4721465&hash=fa30093221863a73960368888463140424019f63b2ef7cefc4743d252d7443c5",
    "checkout_id": "69174757-chred138157fbef-86e3e3f903",
    "currency": "USD",
    "email": "erhan@passwall.io",
    "event_time": "2020-10-07 19:09:57",
    "linked_subscriptions": "",
    "marketing_consent": "0",
    "next_bill_date": "2020-10-21",
    "passthrough": "",
    "quantity": "1",
    "source": "localhost:3000 / localhost:3000",
    "status": "trialing",
    "subscription_id": "4721465",
    "subscription_plan_id": "630862",
    "unit_price": "2.00",
    "update_url": "https://checkout.paddle.com/subscription/update?user=21305808&subscription=4721465&hash=76f88b70fb6273f4cc2ed1414f655dbf837ac7c6db2ab750cbdf0e9068884738",
    "user_id": "21305808",
    "p_signature": "Pnlvy4xL5wmOOgifsJmOpeXEmtzFiHptSs5uY1jMn1tDe+0CZVGRDxNMXQDsJqwUOE3QuR2PvUb0RzPIsH9DmK03ryrZRQW4rVdYqofUW91Fr3I9/nrL/ZstE34VixrEETpb8l0DNlw0rZFYxQrv0mKeRyf9sr2pk3hmoqnpUj9flow660ZjhO+iwQDBgAGAmYRnWXsLlTjISjtb99RxSgRy+Z3LMmSwVj0jauUdAAZXFpNbhkASPhksT2KIPdWmt0OKO5F2O5XtmT4tJ98xDLN9Dr1b0/JWDdfRHrcfWhxkfHGDOmUhosqzbqwF1n3X/FTC07hKTbHJ2LWndtmppDNDGJX5VreZTElJBsW3O27cm7uF9ehWZebJrTs6qTmjoBUC1fMFW4CWzcRDdNqr4QKsaOzyHwtQheWE+iOPF+8BxhTL6pahd/ZKBLgXJ5eCwk7I0n263BP65Jun0bI87Z21FzMU76R0OQj0rpBUVeIgaLafS34Tk572YPMfLnYPYYzDASAegEmnDP87O3QYuXvH3xCUVRBCZvm4qpEWpQ8tXajLvgHq2g+dAmZqg92xFRUb86P45ciK4Z/RBtlCvP/q81AbKfiEEAUV9Vzb3RBa9CEpsAcUFOpU6ESoADS1TbxfLbxkXj1x/DTMhIZ1g8iH1oUBExM1L7TSmzlFHTo="
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
