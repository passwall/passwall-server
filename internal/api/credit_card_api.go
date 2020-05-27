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

const (
	InvalidRequestPayload = "Invalid request payload"
	CreditCardDeleted     = "CreditCard deleted successfully!"
	Success               = "Success"
)

// CreditCardAPI ...
type CreditCardAPI struct {
	CreditCardService app.CreditCardService
}

// NewCreditCardAPI ...
func NewCreditCardAPI(p app.CreditCardService) CreditCardAPI {
	return CreditCardAPI{CreditCardService: p}
}

// GetHandler ...
func (p *CreditCardAPI) GetHandler(w http.ResponseWriter, r *http.Request) {
	action := mux.Vars(r)["action"]

	switch action {
	case "backup":
		app.ListBackup(w, r)
	default:
		common.RespondWithError(w, http.StatusNotFound, InvalidRequestPayload)
		return
	}
}

// FindAll ...
func (p *CreditCardAPI) FindAll(w http.ResponseWriter, r *http.Request) {
	var err error
	var creditCards []model.CreditCard

	fields := []string{"id", "created_at", "updated_at", "bank_name", "bank_code", "account_name", "account_number", "iban", "currency"}
	argsStr, argsInt := SetArgs(r, fields)

	creditCards, err = p.CreditCardService.FindAll(argsStr, argsInt)

	if err != nil {
		common.RespondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	creditCards = app.DecryptCreditCardVerificationNumbers(creditCards)
	common.RespondWithJSON(w, http.StatusOK, creditCards)
}

// FindByID ...
func (p *CreditCardAPI) FindByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		common.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	creditCard, err := p.CreditCardService.FindByID(uint(id))
	if err != nil {
		common.RespondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	passByte, _ := base64.StdEncoding.DecodeString(creditCard.VerificationNumber)
	creditCard.VerificationNumber = string(encryption.Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))

	common.RespondWithJSON(w, http.StatusOK, model.ToCreditCardDTO(creditCard))
}

// Create ...
func (p *CreditCardAPI) Create(w http.ResponseWriter, r *http.Request) {
	var creditCardDTO model.CreditCardDTO

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&creditCardDTO); err != nil {
		common.RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
		return
	}
	defer r.Body.Close()

	rawPass := creditCardDTO.VerificationNumber
	creditCardDTO.VerificationNumber = base64.StdEncoding.EncodeToString(encryption.Encrypt(creditCardDTO.VerificationNumber, viper.GetString("server.passphrase")))

	createdCreditCard, err := p.CreditCardService.Save(model.ToCreditCard(creditCardDTO))
	if err != nil {
		common.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	createdCreditCard.VerificationNumber = rawPass

	common.RespondWithJSON(w, http.StatusOK, model.ToCreditCardDTO(createdCreditCard))
}

// Update ...
func (p *CreditCardAPI) Update(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		common.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	var creditCardDTO model.CreditCardDTO
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&creditCardDTO); err != nil {
		common.RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
		return
	}
	defer r.Body.Close()

	creditCard, err := p.CreditCardService.FindByID(uint(id))
	if err != nil {
		common.RespondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	rawPass := creditCardDTO.VerificationNumber
	creditCardDTO.VerificationNumber = base64.StdEncoding.EncodeToString(encryption.Encrypt(creditCardDTO.VerificationNumber, viper.GetString("server.passphrase")))

	creditCardDTO.ID = uint(id)
	creditCard = model.ToCreditCard(creditCardDTO)
	creditCard.ID = uint(id)

	updatedCreditCard, err := p.CreditCardService.Save(creditCard)
	if err != nil {
		common.RespondWithError(w, http.StatusNotFound, err.Error())
		return
	}
	updatedCreditCard.VerificationNumber = rawPass
	common.RespondWithJSON(w, http.StatusOK, model.ToCreditCardDTO(updatedCreditCard))
}

// Delete ...
func (p *CreditCardAPI) Delete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		common.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	creditCard, err := p.CreditCardService.FindByID(uint(id))
	if err != nil {
		common.RespondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	err = p.CreditCardService.Delete(creditCard.ID)
	if err != nil {
		common.RespondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	response := model.Response{Code: http.StatusOK, Status: Success, Message: CreditCardDeleted}
	common.RespondWithJSON(w, http.StatusOK, response)
}

// Migrate ...
func (p *CreditCardAPI) Migrate() {
	p.CreditCardService.Migrate()
}
