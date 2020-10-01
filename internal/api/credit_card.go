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
	// InvalidRequestPayload represents invalid request payload messaage
	InvalidRequestPayload = "Invalid request payload"
	// CreditCardDeleted represents message when deleting credit cart successfully
	CreditCardDeleted = "CreditCard deleted successfully!"
	// Success represent success message
	Success = "Success"
)

// FindAllCreditCards finds all credid carts
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

// FindCreditCardByID finds a credit cart by id
func FindCreditCardByID(s storage.Store) http.HandlerFunc {
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

		// Decrypt server side encrypted fields
		decCreditCard, err := app.DecryptModel(creditCard)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		creditCardDTO := model.ToCreditCardDTO(decCreditCard.(*model.CreditCard))

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

// CreateCreditCard creates a credit cart
func CreateCreditCard(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		payload, err := ToPayload(r)
		if err != nil {
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

// UpdateCreditCard updates a credit cart
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

// DeleteCreditCard deletes a credit cart
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
			Message: creditCardDeleted,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}
