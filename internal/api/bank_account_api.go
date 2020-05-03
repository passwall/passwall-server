package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/pass-wall/passwall-server/internal/app"
	"github.com/pass-wall/passwall-server/internal/common"
	"github.com/pass-wall/passwall-server/internal/encryption"
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// FindAll ...
func FindAllBankAccounts(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		bankAccounts := []model.BankAccount{}

		fields := []string{"id", "created_at", "updated_at", "bank_name", "bank_code", "account_name", "account_number", "iban", "currency"}
		argsStr, argsInt := SetArgs(r, fields)

		bankAccounts, err = s.BankAccounts().FindAll(argsStr, argsInt)

		if err != nil {
			common.RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		bankAccounts = app.DecryptBankAccountPasswords(bankAccounts)
		common.RespondWithJSON(w, http.StatusOK, bankAccounts)
	}
}

// FindByID ...
func FindBankAccountByID(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			common.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		bankAccount, err := s.BankAccounts().FindByID(uint(id))
		if err != nil {
			common.RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		passByte, _ := base64.StdEncoding.DecodeString(bankAccount.Password)
		bankAccount.Password = string(encryption.Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))

		common.RespondWithJSON(w, http.StatusOK, model.ToBankAccountDTO(bankAccount))
	}
}

// Create ...
func CreateBankAccount(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var bankAccountDTO model.BankAccountDTO

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&bankAccountDTO); err != nil {
			common.RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		if bankAccountDTO.Password == "" {
			bankAccountDTO.Password = encryption.Password()
		}

		rawPass := bankAccountDTO.Password
		bankAccountDTO.Password = base64.StdEncoding.EncodeToString(encryption.Encrypt(bankAccountDTO.Password, viper.GetString("server.passphrase")))

		createdBankAccount, err := s.BankAccounts().Save(model.ToBankAccount(bankAccountDTO))
		if err != nil {
			common.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		createdBankAccount.Password = rawPass

		common.RespondWithJSON(w, http.StatusOK, model.ToBankAccountDTO(createdBankAccount))
	}
}

// Update ...
func UpdateBankAccount(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			common.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		var bankAccountDTO model.BankAccountDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&bankAccountDTO); err != nil {
			common.RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		bankAccount, err := s.BankAccounts().FindByID(uint(id))
		if err != nil {
			common.RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		if bankAccountDTO.Password == "" {
			bankAccountDTO.Password = encryption.Password()
		}
		rawPass := bankAccountDTO.Password
		bankAccountDTO.Password = base64.StdEncoding.EncodeToString(encryption.Encrypt(bankAccountDTO.Password, viper.GetString("server.passphrase")))

		bankAccountDTO.ID = uint(id)
		bankAccount = model.ToBankAccount(bankAccountDTO)
		bankAccount.ID = uint(id)

		updatedBankAccount, err := s.BankAccounts().Save(bankAccount)
		if err != nil {
			common.RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}
		updatedBankAccount.Password = rawPass
		common.RespondWithJSON(w, http.StatusOK, model.ToBankAccountDTO(updatedBankAccount))
	}
}

// Delete ...
func DeleteBankAccount(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			common.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		bankAccount, err := s.BankAccounts().FindByID(uint(id))
		if err != nil {
			common.RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		err = s.BankAccounts().Delete(bankAccount.ID)
		if err != nil {
			common.RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		response := model.Response{http.StatusOK, "Success", "BankAccount deleted successfully!"}
		common.RespondWithJSON(w, http.StatusOK, response)
	}
}
