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
	BankAccountDeleteSuccess = "BankAccount deleted successfully!"
)

// FindAll ...
func FindAllBankAccounts(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var bankAccounts []model.BankAccount

		fields := []string{"id", "created_at", "updated_at", "bank_name", "bank_code", "account_name", "account_number", "iban", "currency"}
		argsStr, argsInt := SetArgs(r, fields)

		bankAccounts, err = s.BankAccounts().FindAll(argsStr, argsInt)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		bankAccounts = app.DecryptBankAccountPasswords(bankAccounts)
		RespondWithJSON(w, http.StatusOK, bankAccounts)
	}
}

// FindByID ...
func FindBankAccountByID(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		account, err := s.BankAccounts().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		bankAccount, err := app.DecryptBankAccountPassword(s, &account)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToBankAccountDTO(bankAccount))
	}
}

// Create ...
func CreateBankAccount(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var bankAccountDTO model.BankAccountDTO

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&bankAccountDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		createdBankAccount, err := app.CreateBankAccount(s, &bankAccountDTO)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToBankAccountDTO(createdBankAccount))
	}
}

// Update ...
func UpdateBankAccount(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		var bankAccountDTO model.BankAccountDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&bankAccountDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		bankAccount, err := s.BankAccounts().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		updatedBankAccount, err := app.UpdateBankAccount(s, &bankAccount, &bankAccountDTO)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToBankAccountDTO(updatedBankAccount))
	}
}

// Delete ...
func DeleteBankAccount(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		bankAccount, err := s.BankAccounts().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		err = s.BankAccounts().Delete(bankAccount.ID)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		response := model.Response{
			Code:    http.StatusOK,
			Status:  Success,
			Message: BankAccountDeleteSuccess,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}
