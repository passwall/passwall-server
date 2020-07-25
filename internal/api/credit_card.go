package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"

	"github.com/gorilla/mux"
)

const (
	InvalidRequestPayload = "Invalid request payload"
	CreditCardDeleted     = "CreditCard deleted successfully!"
	Success               = "Success"
)

// FindAll ...
func FindAllCreditCards(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var creditCards []model.CreditCard

		fields := []string{"id", "created_at", "updated_at", "bank_name", "bank_code", "account_name", "account_number", "iban", "currency"}
		argsStr, argsInt := SetArgs(r, fields)

		schema := r.Context().Value("schema").(string)
		creditCards, err = s.CreditCards().FindAll(argsStr, argsInt, schema)

		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// creditCards = app.DecryptCreditCardVerificationNumbers(creditCards)

		// Encrypt payload
		var payload model.Payload
		key := r.Context().Value("transmissionKey").(string)
		encrypted, err := app.EncryptJSON(key, creditCards)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// FindByID ...
func FindCreditCardByID(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		schema := r.Context().Value("schema").(string)
		card, err := s.CreditCards().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		creditCard, err := app.DecryptCreditCardVerificationNumber(s, card)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		creditCardDTO := model.ToCreditCardDTO(creditCard)

		// Encrypt payload
		var payload model.Payload
		key := r.Context().Value("transmissionKey").(string)
		encrypted, err := app.EncryptJSON(key, creditCardDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// Create ...
func CreateCreditCard(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// TODO BEGIN: This part should be in a helper function
		// Unmarshal request body to payload
		var payload model.Payload
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payload); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()
		// TODO END:

		// Decrypt payload
		var creditCardDTO model.CreditCardDTO
		key := r.Context().Value("transmissionKey").(string)
		err := app.DecryptJSON(key, []byte(payload.Data), &creditCardDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		schema := r.Context().Value("schema").(string)
		createdCreditCard, err := app.CreateCreditCard(s, &creditCardDTO, schema)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		createdCreditCardDTO := model.ToCreditCardDTO(createdCreditCard)

		// Encrypt payload
		encrypted, err := app.EncryptJSON(key, createdCreditCardDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// Update ...
func UpdateCreditCard(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Unmarshal request body to payload
		var payload model.Payload
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payload); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		// Decrypt payload
		var creditCardDTO model.CreditCardDTO
		key := r.Context().Value("transmissionKey").(string)
		err = app.DecryptJSON(key, []byte(payload.Data), &creditCardDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		schema := r.Context().Value("schema").(string)
		creditCard, err := s.CreditCards().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		updatedCreditCard, err := app.UpdateCreditCard(s, creditCard, &creditCardDTO, schema)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		updatedCreditCardDTO := model.ToCreditCardDTO(updatedCreditCard)

		// Encrypt payload
		encrypted, err := app.EncryptJSON(key, updatedCreditCardDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// Delete ...
func DeleteCreditCard(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		schema := r.Context().Value("schema").(string)
		creditCard, err := s.CreditCards().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		err = s.CreditCards().Delete(creditCard.ID, schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		response := model.Response{
			Code:    http.StatusOK,
			Status:  Success,
			Message: CreditCardDeleted,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}
