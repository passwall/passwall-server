package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/pass-wall/passwall-server/internal/app"
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"

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

		creditCards, err = s.CreditCards().FindAll(argsStr, argsInt)

		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		creditCards = app.DecryptCreditCardVerificationNumbers(creditCards)
		RespondWithJSON(w, http.StatusOK, creditCards)
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

		card, err := s.CreditCards().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		creditCard, err := app.DecryptCreditCardVerificationNumber(s, &card)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToCreditCardDTO(creditCard))
	}
}

// Create ...
func CreateCreditCard(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var creditCardDTO model.CreditCardDTO

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&creditCardDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		createdCreditCard, err := app.CreateCreditCard(s, &creditCardDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToCreditCardDTO(createdCreditCard))
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

		var creditCardDTO model.CreditCardDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&creditCardDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		creditCard, err := s.CreditCards().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		updatedCreditCard, err := app.UpdateCreditCard(s, &creditCard, &creditCardDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToCreditCardDTO(updatedCreditCard))
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

		creditCard, err := s.CreditCards().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		err = s.CreditCards().Delete(creditCard.ID)
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
