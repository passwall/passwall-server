package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	"github.com/spf13/viper"

	"github.com/gorilla/mux"
)

const (
	SubscriptionDeleteSuccess = "Subscription deleted successfully!"
)

// PostSubscription ...
func PostSubscription(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// 0. API Key Check
		keys, ok := r.URL.Query()["api_key"]

		if !ok || len(keys[0]) < 1 {
			RespondWithError(w, http.StatusBadRequest, "API Key is missing")
			return
		}

		if keys[0] != viper.GetString("server.apiKey") {
			RespondWithError(w, http.StatusUnauthorized, "API Key is wrong")
			return
		}

		subHook := new(model.SubscriptionHook)
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&subHook); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		var code int
		var msg string

		switch subHook.AlertName {
		case "subscription_created":
			code, msg = app.CreateSubscription(s, subHook)
		case "subscription_updated":
			code, msg = app.UpdateSubscription(s, subHook)
		case "subscription_cancelled":
			code, msg = app.CancelSubscription(s, subHook)
		case "subscription_payment_succeeded":
			code, msg = app.PaymentSucceedSubscription(s, subHook)
		case "subscription_payment_failed":
			code, msg = app.PaymentFailedSubscription(s, subHook)
		default:
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest")
			return
		}

		if code != http.StatusOK {
			RespondWithError(w, code, msg)
			return
		}

		response := model.Response{
			Code:    http.StatusOK,
			Status:  Success,
			Message: msg,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}

// FindAllSubscriptions ...
func FindAllSubscriptions(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var subscriptionList []model.Subscription

		fields := []string{"id", "created_at", "updated_at", "title", "ip", "url"}
		argsStr, argsInt := SetArgs(r, fields)

		subscriptionList, err = s.Subscriptions().FindAll(argsStr, argsInt)

		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Encrypt payload
		var payload model.Payload
		key := r.Context().Value("transmissionKey").(string)
		encrypted, err := app.EncryptJSON(key, subscriptionList)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// FindSubscriptionByID ...
func FindSubscriptionByID(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		subscription, err := s.Subscriptions().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Decrypt subscription side encrypted fields
		decSubscription, err := app.DecryptModel(subscription)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		subscriptionDTO := model.ToSubscriptionDTO(decSubscription.(*model.Subscription))

		// Encrypt payload
		var payload model.Payload
		key := r.Context().Value("transmissionKey").(string)
		encrypted, err := app.EncryptJSON(key, subscriptionDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// CreateSubscription ...
func CreateSubscription(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		payload, err := ToPayload(r)
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		// Decrypt payload
		var subscriptionDTO model.SubscriptionDTO
		key := r.Context().Value("transmissionKey").(string)
		err = app.DecryptJSON(key, []byte(payload.Data), &subscriptionDTO)
		if err != nil {
			fmt.Println("burada")
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		createdSubscription, err := s.Subscriptions().Save(model.ToSubscription(&subscriptionDTO))
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		createdSubscriptionDTO := model.ToSubscriptionDTO(createdSubscription)

		// Encrypt payload
		encrypted, err := app.EncryptJSON(key, createdSubscriptionDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// UpdateSubscription ...
// func UpdateSubscription(s storage.Store) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		vars := mux.Vars(r)
// 		id, err := strconv.Atoi(vars["id"])
// 		if err != nil {
// 			RespondWithError(w, http.StatusBadRequest, err.Error())
// 			return
// 		}

// 		// Unmarshal request body to payload
// 		var payload model.Payload
// 		decoder := json.NewDecoder(r.Body)
// 		if err := decoder.Decode(&payload); err != nil {
// 			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
// 			return
// 		}
// 		defer r.Body.Close()

// 		// Decrypt payload
// 		var subscriptionDTO model.SubscriptionDTO
// 		key := r.Context().Value("transmissionKey").(string)
// 		err = app.DecryptJSON(key, []byte(payload.Data), &subscriptionDTO)
// 		if err != nil {
// 			RespondWithError(w, http.StatusInternalServerError, err.Error())
// 			return
// 		}

// 		schema := "public"
// 		subscription, err := s.Subscriptions().FindByID(uint(id), schema)
// 		if err != nil {
// 			RespondWithError(w, http.StatusNotFound, err.Error())
// 			return
// 		}

// 		updatedSubscription, err := s.Subscriptions().Save(model.ToSubscription(&subscriptionDTO), schema) app.UpdateSubscription(s, subscription, &subscriptionDTO, schema)
// 		if err != nil {
// 			RespondWithError(w, http.StatusInternalServerError, err.Error())
// 			return
// 		}

// 		updatedSubscriptionDTO := model.ToSubscriptionDTO(updatedSubscription)

// 		// Encrypt payload
// 		encrypted, err := app.EncryptJSON(key, updatedSubscriptionDTO)
// 		if err != nil {
// 			RespondWithError(w, http.StatusInternalServerError, err.Error())
// 			return
// 		}
// 		payload.Data = string(encrypted)

// 		RespondWithJSON(w, http.StatusOK, payload)
// 	}
// }

// DeleteSubscription ...
func DeleteSubscription(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		subscription, err := s.Subscriptions().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		err = s.Subscriptions().Delete(subscription.ID)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		response := model.Response{
			Code:    http.StatusOK,
			Status:  Success,
			Message: SubscriptionDeleteSuccess,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}
