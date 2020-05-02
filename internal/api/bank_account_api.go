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
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// BankAccountAPI ...
type BankAccountAPI struct {
	BankAccountService app.BankAccountService
}

// NewBankAccountAPI ...
func NewBankAccountAPI(p app.BankAccountService) BankAccountAPI {
	return BankAccountAPI{BankAccountService: p}
}

// GetHandler ...
func (p *BankAccountAPI) GetHandler(w http.ResponseWriter, r *http.Request) {
	action := mux.Vars(r)["action"]

	switch action {
	case "backup":
		app.ListBackup(w, r)
	default:
		common.RespondWithError(w, http.StatusNotFound, "Invalid resquest payload")
		return
	}
}

// FindAll ...
func (p *BankAccountAPI) FindAll(w http.ResponseWriter, r *http.Request) {
	var err error
	bankAccounts := []model.BankAccount{}

	argsStr, argsInt := SetArgs(r)

	bankAccounts, err = p.BankAccountService.FindAll(argsStr, argsInt)

	if err != nil {
		common.RespondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	bankAccounts = app.DecryptBankAccountPasswords(bankAccounts)
	common.RespondWithJSON(w, http.StatusOK, bankAccounts)
}

// FindByID ...
func (p *BankAccountAPI) FindByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		common.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	bankAccount, err := p.BankAccountService.FindByID(uint(id))
	if err != nil {
		common.RespondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	passByte, _ := base64.StdEncoding.DecodeString(bankAccount.Password)
	bankAccount.Password = string(encryption.Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))

	common.RespondWithJSON(w, http.StatusOK, model.ToBankAccountDTO(bankAccount))
}

// Create ...
func (p *BankAccountAPI) Create(w http.ResponseWriter, r *http.Request) {
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

	createdBankAccount, err := p.BankAccountService.Save(model.ToBankAccount(bankAccountDTO))
	if err != nil {
		common.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	createdBankAccount.Password = rawPass

	common.RespondWithJSON(w, http.StatusOK, model.ToBankAccountDTO(createdBankAccount))
}

// Update ...
func (p *BankAccountAPI) Update(w http.ResponseWriter, r *http.Request) {
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

	bankAccount, err := p.BankAccountService.FindByID(uint(id))
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

	updatedBankAccount, err := p.BankAccountService.Save(bankAccount)
	if err != nil {
		common.RespondWithError(w, http.StatusNotFound, err.Error())
		return
	}
	updatedBankAccount.Password = rawPass
	common.RespondWithJSON(w, http.StatusOK, model.ToBankAccountDTO(updatedBankAccount))
}

// Delete ...
func (p *BankAccountAPI) Delete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		common.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	bankAccount, err := p.BankAccountService.FindByID(uint(id))
	if err != nil {
		common.RespondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	err = p.BankAccountService.Delete(bankAccount.ID)
	if err != nil {
		common.RespondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	response := model.Response{http.StatusOK, "Success", "BankAccount deleted successfully!"}
	common.RespondWithJSON(w, http.StatusOK, response)
}

// Migrate ...
func (p *BankAccountAPI) Migrate() {
	p.BankAccountService.Migrate()
}
