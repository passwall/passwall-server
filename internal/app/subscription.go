package app

import (
	"net/http"
	"strconv"
	"time"

	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
)

//CreateSubscription creates a subscription and saves it to the store
func CreateSubscription(s storage.Store, r *http.Request) (int, string) {

	subID, err := strconv.Atoi(r.FormValue("subscription_id"))
	if err != nil {
		return http.StatusBadRequest, err.Error()
	}

	_, err = s.Subscriptions().FindBySubscriptionID(uint(subID))
	if err == nil {
		message := "Subscription already exist!"
		return http.StatusBadRequest, message
	}

	_, err = s.Subscriptions().Save(model.RequestToSub(r))
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	return http.StatusOK, "Subscription created successfully."
}

//UpdateSubscription updates a subscription
func UpdateSubscription(s storage.Store, bodyMap map[string]string) (int, string) {
	subID, err := strconv.Atoi(bodyMap["subscription_id"])
	if err != nil {
		return http.StatusBadRequest, err.Error()
	}

	planID, err := strconv.Atoi(r.FormValue("subscription_plan_id"))
	if err != nil {
		return http.StatusBadRequest, err.Error()
	}

	nextBillDate, err := time.Parse("2006-01-02", r.FormValue("next_bill_date"))
	if err != nil {
		return http.StatusBadRequest, err.Error()
	}

	subscription, err := s.Subscriptions().FindBySubscriptionID(uint(subID))
	if err != nil {
		return http.StatusNotFound, err.Error()
	}

	subscription.PlanID = planID
	subscription.NextBillDate = nextBillDate
	subscription.Status = r.FormValue("status")

	_, err = s.Subscriptions().Save(subscription)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	return http.StatusOK, "Subscription updated successfully."
}

//CancelSubscription cancels a subscription
func CancelSubscription(s storage.Store, bodyMap map[string]string) (int, string) {
	subID, err := strconv.Atoi(bodyMap["subscription_id"])
	if err != nil {
		return http.StatusBadRequest, err.Error()
	}

	nextBillDate, err := time.Parse("2006-01-02", "0001-01-01")
	if err != nil {
		return http.StatusBadRequest, err.Error()
	}

	subscription, err := s.Subscriptions().FindBySubscriptionID(uint(subID))
	if err != nil {
		return http.StatusNotFound, err.Error()
	}

	subscription.NextBillDate = nextBillDate
	subscription.Status = r.FormValue("status")
	subscription.CancelledAt = time.Now()

	_, err = s.Subscriptions().Save(subscription)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	return http.StatusOK, "Subscription cancelled."
}

//PaymentSucceedSubscription payment succeed for subscription
func PaymentSucceedSubscription(s storage.Store, bodyMap map[string]string) (int, string) {
	subID, err := strconv.Atoi(bodyMap["subscription_id"])
	if err != nil {
		return http.StatusBadRequest, err.Error()
	}

	nextBillDate, err := time.Parse("2006-01-02", r.FormValue("next_bill_date"))
	if err != nil {
		return http.StatusBadRequest, err.Error()
	}

	subscription, err := s.Subscriptions().FindBySubscriptionID(uint(subID))
	if err != nil {
		return http.StatusNotFound, err.Error()
	}

	subscription.NextBillDate = nextBillDate

	_, err = s.Subscriptions().Save(subscription)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	return http.StatusOK, "Subscription payment succeeded."
}

//PaymentFailedSubscription payment failed for subscription
func PaymentFailedSubscription(s storage.Store, bodyMap map[string]string) (int, string) {
	subID, err := strconv.Atoi(bodyMap["subscription_id"])
	if err != nil {
		return http.StatusBadRequest, err.Error()
	}

	nextBillDate, err := time.Parse("2006-01-02", "0001-01-01")
	if err != nil {
		return http.StatusBadRequest, err.Error()
	}

	subscription, err := s.Subscriptions().FindBySubscriptionID(uint(subID))
	if err != nil {
		return http.StatusNotFound, err.Error()
	}

	subscription.NextBillDate = nextBillDate
	subscription.Status = "past_due"

	_, err = s.Subscriptions().Save(subscription)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	return http.StatusOK, "Subscription payment failed."
}
