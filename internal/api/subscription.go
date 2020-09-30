package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"

	"github.com/gorilla/mux"
)

const (
	SubscriptionDeleteSuccess = "Subscription deleted successfully!"
)

func getAlertName(r *http.Request) (string, error) {
	alertNameDTO := new(model.AlertName)

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&alertNameDTO); err != nil {
		return "", err
	}
	// defer r.Body.Close()

	return alertNameDTO.AlertName, nil
}

// PostSubscription ...
func PostSubscription(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		bodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}

		bodyMap := make(map[string]string)
		err = json.Unmarshal(bodyBytes, &bodyMap)
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}

		r.Body.Close() //  must close

		// Generate body again for alert type
		r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

		if bodyMap["alert_name"] == "subscription_created" {
			subscriptionCreated := new(model.SubscriptionCreated)

			decoder := json.NewDecoder(r.Body)
			if err := decoder.Decode(&subscriptionCreated); err != nil {
				RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
				return
			}
			defer r.Body.Close()

			subID, err := strconv.Atoi(subscriptionCreated.SubscriptionID)
			if err != nil {
				RespondWithError(w, http.StatusBadRequest, err.Error())
				return
			}

			_, err = s.Subscriptions().FindBySubscriptionID(uint(subID))
			if err == nil {
				message := "Subscription already exist!"
				RespondWithError(w, http.StatusBadRequest, message)
				return
			}

			_, err = s.Subscriptions().Save(model.FromCreToSub(subscriptionCreated))
			if err != nil {
				RespondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		if bodyMap["alert_name"] == "subscription_updated" {
			planID, err := strconv.Atoi(bodyMap["subscription_plan_id"])
			if err != nil {
				RespondWithError(w, http.StatusBadRequest, err.Error())
				return
			}

			subID, err := strconv.Atoi(bodyMap["subscription_id"])
			if err != nil {
				RespondWithError(w, http.StatusBadRequest, err.Error())
				return
			}

			nextBillDate, err := time.Parse("2006-01-02", bodyMap["next_bill_date"])
			if err != nil {
				RespondWithError(w, http.StatusBadRequest, err.Error())
				return
			}

			subscription, err := s.Subscriptions().FindBySubscriptionID(uint(subID))
			if err != nil {
				message := "Subscription is not exist!"
				RespondWithError(w, http.StatusNotFound, message)
				return
			}

			subscription.PlanID = planID
			subscription.NextBillDate = nextBillDate
			subscription.Status = bodyMap["status"]

			_, err = s.Subscriptions().Save(subscription)
			if err != nil {
				RespondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}

		}

		if bodyMap["alert_name"] == "subscription_cancelled" {
			subID, err := strconv.Atoi(bodyMap["subscription_id"])
			if err != nil {
				RespondWithError(w, http.StatusBadRequest, err.Error())
				return
			}

			nextBillDate, err := time.Parse("2006-01-02", "0001-01-01")
			if err != nil {
				RespondWithError(w, http.StatusBadRequest, err.Error())
				return
			}

			subscription, err := s.Subscriptions().FindBySubscriptionID(uint(subID))
			if err != nil {
				message := "Subscription is not exist!"
				RespondWithError(w, http.StatusNotFound, message)
				return
			}

			subscription.NextBillDate = nextBillDate
			subscription.Status = bodyMap["status"]
			subscription.CancelledAt = time.Now()

			_, err = s.Subscriptions().Save(subscription)
			if err != nil {
				RespondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}

		}
		// case 'subscription_updated':
		// // The next billing date of this user's subscription.
		// $next_bill_date = $db->real_escape_string($_POST['next_bill_date']);

		// $db->query("UPDATE subscriptions SET next_bill_date = '$next_bill_date', plan_id = '$plan_id', status = '$status' WHERE subscription_id = '$subscription_id'");

		// break;

		//subscriptionDTO := new(AlertName)

		// TODO: There are 6 action here. These should be moved to service layer
		// user's service layer functions located in /app/user.go file is

		// 1. Decode request body to userDTO object
		// decoder := json.NewDecoder(r.Body)
		// if err := decoder.Decode(&subscriptionDTO); err != nil {
		// 	RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
		// 	return
		// }
		// defer r.Body.Close()

		// j, _ := json.Marshal(subscriptionDTO)
		// fmt.Println(string(j))

		// schema := "public"
		// createdSubscription, err := s.Subscriptions().Save(model.ToSubscription(&subscriptionDTO), schema)
		// if err != nil {
		// 	RespondWithError(w, http.StatusInternalServerError, err.Error())
		// 	return
		// }

		// createdSubscriptionDTO := model.ToSubscriptionDTO(createdSubscription)

		response := model.Response{
			Code:    http.StatusOK,
			Status:  Success,
			Message: "Subscription created/updated successfully.",
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
