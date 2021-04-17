package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	"github.com/spf13/viper"

	"github.com/gorilla/mux"
)

const (
	bankAccountDeleteSuccess = "BankAccount deleted successfully!"
)

// FindAllBankAccounts finds all bank accounts
func FindAllBankAccounts(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		argsStr, argsInt := SetArgs(r,
			[]string{
				"id",
				"created_at",
				"updated_at",
				"bank_name",
				"bank_code",
				"account_name",
				"account_number",
				"iban",
				"currency"})

		// Get all bank accounts from db
		bankAccountList, err := s.BankAccounts().FindAll(argsStr, argsInt, r.Context().Value("schema").(string))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Decrypt server side encrypted fields
		for i := range bankAccountList {
			uBankAccount, err := app.DecryptModel(&bankAccountList[i])
			if err != nil {
				RespondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}
			bankAccountList[i] = *uBankAccount.(*model.BankAccount)
		}

		RespondWithEncJSON(w, http.StatusOK, r.Context().Value("transmissionKey").(string), bankAccountList)
	}
}

// FindBankAccountByID finds a bank account by id
func FindBankAccountByID(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if id is integer
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Find login by id from db
		schema := r.Context().Value("schema").(string)
		bankAccount, err := s.BankAccounts().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Decrypt server side encrypted fields
		uBankAccount, err := app.DecryptModel(bankAccount)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithEncJSON(
			w,
			http.StatusOK,
			r.Context().Value("transmissionKey").(string),
			model.ToBankAccountDTO(uBankAccount.(*model.BankAccount)))
	}
}

// CreateBankAccount creates a bank aaccount
func CreateBankAccount(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Setup variables
		transmissionKey := r.Context().Value("transmissionKey").(string)

		// Update request body according to env.
		// If env is dev, then do nothing
		// If env is prod, then decrypt payload with transmission key
		if err := ToBody(r, viper.GetString("server.env"), transmissionKey); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		// Unmarshal request body to bankAccountDTO
		var bankAccountDTO model.BankAccountDTO
		if err := json.NewDecoder(r.Body).Decode(&bankAccountDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		// Add new bankaccount to db
		createdBankAccount, err := app.CreateBankAccount(s, &bankAccountDTO, r.Context().Value("schema").(string))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Decrypt server side encrypted fields
		decBankAccount, err := app.DecryptModel(createdBankAccount)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithEncJSON(
			w,
			http.StatusOK,
			transmissionKey,
			model.ToBankAccountDTO(decBankAccount.(*model.BankAccount)))
	}
}

// UpdateBankAccount updates a bank account
func UpdateBankAccount(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(mux.Vars(r)["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Setup variables
		transmissionKey := r.Context().Value("transmissionKey").(string)

		if err := ToBody(r, viper.GetString("server.env"), transmissionKey); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		// Unmarshal request body to loginDTO
		var bankAccountDTO model.BankAccountDTO
		if err := json.NewDecoder(r.Body).Decode(&bankAccountDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		// Find login defined by id
		schema := r.Context().Value("schema").(string)
		bankAccount, err := s.BankAccounts().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Update login
		updatedBankAccount, err := app.UpdateBankAccount(s, bankAccount, &bankAccountDTO, schema)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Decrypt server side encrypted fields
		decBankAccount, err := app.DecryptModel(updatedBankAccount)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithEncJSON(
			w,
			http.StatusOK,
			transmissionKey,
			model.ToBankAccountDTO(decBankAccount.(*model.BankAccount)))
	}
}

// DeleteBankAccount deletes a bank account
func DeleteBankAccount(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(mux.Vars(r)["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		schema := r.Context().Value("schema").(string)
		bankAccount, err := s.BankAccounts().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		err = s.BankAccounts().Delete(bankAccount.ID, schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK,
			model.Response{
				Code:    http.StatusOK,
				Status:  Success,
				Message: bankAccountDeleteSuccess,
			})
	}
}
