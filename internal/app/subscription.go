package app

import (
	"net/http"
	"strconv"
	"time"

	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
)

// CreateSubscription creates a subscription and saves it to the store
func CreateSubscription(s storage.Store, r *http.Request) (int, string) {
	_, err := s.Subscriptions().FindByEmail(r.FormValue("email"))
	if err == nil {
		message := "Subscription already exist!"
		return http.StatusBadRequest, message
	}

	_, err = s.Subscriptions().Create(model.RequestToSub(r))
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	return http.StatusOK, "Subscription created successfully."
}

func UpdateSubscription(s storage.Store, r *http.Request) (int, string) {
	subscription, err := s.Subscriptions().FindByEmail(r.FormValue("email"))
	if err != nil {
		return http.StatusNotFound, err.Error()
	}

	subscriptionID, err := strconv.Atoi(r.FormValue("subscription_id"))
	if err != nil {
		return http.StatusBadRequest, err.Error()
	}

	planID, err := strconv.Atoi(r.FormValue("subscription_plan_id"))
	if err != nil {
		return http.StatusBadRequest, err.Error()
	}

	userID, err := strconv.Atoi(r.FormValue("user_id"))
	if err != nil {
		return http.StatusBadRequest, err.Error()
	}

	nextBillDate, err := time.Parse("2006-01-02", r.FormValue("next_bill_date"))
	if err != nil {
		return http.StatusBadRequest, err.Error()
	}

	subscription.Type = "pro"
	subscription.SubscriptionID = subscriptionID
	subscription.PlanID = planID
	subscription.UserID = userID
	subscription.NextBillDate = nextBillDate
	subscription.Status = r.FormValue("status")

	_, err = s.Subscriptions().Update(subscription)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	return http.StatusOK, "Subscription updated successfully."
}

func CancelSubscription(s storage.Store, r *http.Request) (int, string) {
	subscription, err := s.Subscriptions().FindByEmail(r.FormValue("email"))
	if err != nil {
		return http.StatusNotFound, err.Error()
	}

	err = s.Subscriptions().Delete(subscription.ID)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	return http.StatusOK, "Subscription cancelled."
}

func PaymentSucceedSubscription(s storage.Store, r *http.Request) (int, string) {
	subscription, err := s.Subscriptions().FindByEmail(r.FormValue("email"))
	if err != nil {
		return http.StatusNotFound, err.Error()
	}

	nextBillDate, err := time.Parse("2006-01-02", r.FormValue("next_bill_date"))
	if err != nil {
		return http.StatusBadRequest, err.Error()
	}

	subscription.NextBillDate = nextBillDate

	_, err = s.Subscriptions().Update(subscription)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	return http.StatusOK, "Subscription payment succeeded."
}

func PaymentFailedSubscription(s storage.Store, r *http.Request) (int, string) {
	subscription, err := s.Subscriptions().FindByEmail(r.FormValue("email"))
	if err != nil {
		return http.StatusNotFound, err.Error()
	}

	nextBillDate, err := time.Parse("2006-01-02", "0001-01-01")
	if err != nil {
		return http.StatusBadRequest, err.Error()
	}

	subscription.NextBillDate = nextBillDate
	subscription.Status = r.FormValue("status")

	_, err = s.Subscriptions().Update(subscription)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	return http.StatusOK, "Subscription payment failed."
}
