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
		var creditCardList []model.CreditCard

		// Get all credit cards from db
		schema := r.Context().Value("schema").(string)
		creditCardList, err = s.CreditCards().All(schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Decrypt server side encrypted fields
		for i := range creditCardList {
			uCreditCard, err := app.DecryptModel(&creditCardList[i])
			if err != nil {
				RespondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}
			creditCardList[i] = *uCreditCard.(*model.CreditCard)
		}

		RespondWithJSON(w, http.StatusOK, creditCardList)
	}
}

// FindCreditCardByID finds a credit cart by id
func FindCreditCardByID(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Check if id is integer
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Find credit card by id from db
		schema := r.Context().Value("schema").(string)
		creditCard, err := s.CreditCards().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Decrypt server side encrypted fields
		uCreditCard, err := app.DecryptModel(creditCard)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Create DTO
		creditCardDTO := model.ToCreditCardDTO(uCreditCard.(*model.CreditCard))

		RespondWithJSON(w, http.StatusOK, creditCardDTO)
	}
}

// CreateCreditCard creates a credit cart
func CreateCreditCard(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Unmarshal request body to creditCardDTO
		var creditCardDTO model.CreditCardDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&creditCardDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		// Add new credit card to db
		schema := r.Context().Value("schema").(string)
		createdCreditCard, err := app.CreateCreditCard(s, &creditCardDTO, schema)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Decrypt server side encrypted fields
		decCreditCard, err := app.DecryptModel(createdCreditCard)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Create DTO
		createdCreditCardDTO := model.ToCreditCardDTO(decCreditCard.(*model.CreditCard))

		RespondWithJSON(w, http.StatusOK, createdCreditCardDTO)
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

		// Unmarshal request body to creditCardDTO
		var creditCardDTO model.CreditCardDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&creditCardDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		// Find credit card defined by id
		schema := r.Context().Value("schema").(string)
		creditCard, err := s.CreditCards().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Update credit card
		updatedCreditCard, err := app.UpdateCreditCard(s, creditCard, &creditCardDTO, schema)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Decrypt server side encrypted fields
		decCreditCard, err := app.DecryptModel(updatedCreditCard)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Create DTO
		updatedCreditCardDTO := model.ToCreditCardDTO(decCreditCard.(*model.CreditCard))

		RespondWithJSON(w, http.StatusOK, updatedCreditCardDTO)
	}
}

// BulkUpdateCreditCards updates creditCards in payload
func BulkUpdateCreditCards(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var creditCardList []model.CreditCardDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&creditCardList); err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		defer r.Body.Close()

		for _, creditCardDTO := range creditCardList {
			// Find creditCard defined by id
			schema := r.Context().Value("schema").(string)
			creditCard, err := s.CreditCards().FindByID(creditCardDTO.ID, schema)
			if err != nil {
				RespondWithError(w, http.StatusNotFound, err.Error())
				return
			}

			// Update creditCard
			_, err = app.UpdateCreditCard(s, creditCard, &creditCardDTO, schema)
			if err != nil {
				RespondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		response := model.Response{
			Code:    http.StatusOK,
			Status:  "Success",
			Message: "Bulk update completed successfully!",
		}
		RespondWithJSON(w, http.StatusOK, response)
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
			Message: CreditCardDeleted,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}
