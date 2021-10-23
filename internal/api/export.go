package api

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"net/http"

	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
)

func getLogins(s storage.Store, w http.ResponseWriter, r *http.Request) [][]string {
	var err error
	var loginList []model.Login

	fields := []string{"id", "created_at", "updated_at", "title"}
	argsStr, argsInt := SetArgs(r, fields)
	schema := r.Context().Value("schema").(string)

	// Get all logins from db
	loginList, err = s.Logins().FindAll(argsStr, argsInt, schema)
	if err != nil {
		RespondWithError(w, http.StatusNotFound, err.Error())
		return nil
	}

	// Decrypt server side encrypted fields
	for i := range loginList {
		uLogin, err := app.DecryptModel(&loginList[i])
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return nil
		}
		loginList[i] = *uLogin.(*model.Login)
	}

	var content [][]string
	content = append(content, []string{"URL", "Username", "Password"})
	for i := range loginList {
		content = append(content, []string{loginList[i].URL, loginList[i].Username, loginList[i].Password})
	}

	return content
}

func getBankAccounts(s storage.Store, w http.ResponseWriter, r *http.Request) [][]string {
	var err error
	var bankAccountList []model.BankAccount

	fields := []string{"id", "created_at", "updated_at", "bank_name", "bank_code", "account_name", "account_number", "iban", "currency"}
	argsStr, argsInt := SetArgs(r, fields)
	schema := r.Context().Value("schema").(string)

	// Get all bank accounts from db
	bankAccountList, err = s.BankAccounts().FindAll(argsStr, argsInt, schema)
	if err != nil {
		RespondWithError(w, http.StatusNotFound, err.Error())
		return nil
	}

	// Decrypt server side encrypted fields
	for i := range bankAccountList {
		uBankAccount, err := app.DecryptModel(&bankAccountList[i])
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return nil
		}
		bankAccountList[i] = *uBankAccount.(*model.BankAccount)
	}

	var content [][]string
	content = append(content, []string{"BankName", "BankCode", "AccountName", "AccountNumber", "IBAN", "Currency", "Password"})
	for i := range bankAccountList {
		content = append(content, []string{bankAccountList[i].BankName,
			bankAccountList[i].BankCode, bankAccountList[i].AccountName,
			bankAccountList[i].AccountNumber, bankAccountList[i].IBAN,
			bankAccountList[i].Currency, bankAccountList[i].Password})
	}

	return content
}

func getCreditCards(s storage.Store, w http.ResponseWriter, r *http.Request) [][]string {
	var err error
	var creditCardList []model.CreditCard

	fields := []string{"id", "created_at", "updated_at", "bank_name", "bank_code", "account_name", "account_number", "iban", "currency"}
	argsStr, argsInt := SetArgs(r, fields)
	schema := r.Context().Value("schema").(string)

	// Get all credit cards from db
	creditCardList, err = s.CreditCards().FindAll(argsStr, argsInt, schema)
	if err != nil {
		RespondWithError(w, http.StatusNotFound, err.Error())
		return nil
	}

	// Decrypt server side encrypted fields
	for i := range creditCardList {
		uCreditCard, err := app.DecryptModel(&creditCardList[i])
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return nil
		}
		creditCardList[i] = *uCreditCard.(*model.CreditCard)
	}

	var content [][]string
	content = append(content, []string{"CardName", "CardholderName", "Type", "Number", "VerificationNumber", "ExpiryDate"})
	for i := range creditCardList {
		content = append(content, []string{creditCardList[i].CardName,
			creditCardList[i].CardholderName, creditCardList[i].Type,
			creditCardList[i].Number, creditCardList[i].VerificationNumber,
			creditCardList[i].ExpiryDate})
	}

	return content
}

func getEmails(s storage.Store, w http.ResponseWriter, r *http.Request) [][]string {
	var err error
	var emailList []model.Email

	fields := []string{"id", "created_at", "updated_at", "email"}
	argsStr, argsInt := SetArgs(r, fields)
	schema := r.Context().Value("schema").(string)

	// Get all emails from db
	emailList, err = s.Emails().FindAll(argsStr, argsInt, schema)
	if err != nil {
		RespondWithError(w, http.StatusNotFound, err.Error())
		return nil
	}

	// Decrypt server side encrypted fields
	for i := range emailList {
		decEmail, err := app.DecryptModel(&emailList[i])
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return nil
		}
		emailList[i] = *decEmail.(*model.Email)
	}

	var content [][]string
	content = append(content, []string{"Title", "Email", "Password"})
	for i := range emailList {
		content = append(content, []string{emailList[i].Title, emailList[i].Email, emailList[i].Password})
	}

	return content
}

func getNotes(s storage.Store, w http.ResponseWriter, r *http.Request) [][]string {
	var err error
	var noteList []model.Note

	fields := []string{"id", "created_at", "updated_at", "note"}
	argsStr, argsInt := SetArgs(r, fields)
	schema := r.Context().Value("schema").(string)

	// Get all notes from db
	noteList, err = s.Notes().FindAll(argsStr, argsInt, schema)
	if err != nil {
		RespondWithError(w, http.StatusNotFound, err.Error())
		return nil
	}

	// Decrypt server side encrypted fields
	for i := range noteList {
		uNote, err := app.DecryptModel(&noteList[i])
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return nil
		}
		noteList[i] = *uNote.(*model.Note)
	}

	var content [][]string
	content = append(content, []string{"Title", "Note"})
	for i := range noteList {
		content = append(content, []string{noteList[i].Title, noteList[i].Note})
	}

	return content
}

func getServers(s storage.Store, w http.ResponseWriter, r *http.Request) [][]string {
	var err error
	var serverList []model.Server

	fields := []string{"id", "created_at", "updated_at", "title", "ip", "url"}
	argsStr, argsInt := SetArgs(r, fields)
	schema := r.Context().Value("schema").(string)

	// Get all servers from db
	serverList, err = s.Servers().FindAll(argsStr, argsInt, schema)
	if err != nil {
		RespondWithError(w, http.StatusNotFound, err.Error())
		return nil
	}

	// Decrypt server side encrypted fields
	for i := range serverList {
		decServer, err := app.DecryptModel(&serverList[i])
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return nil
		}
		serverList[i] = *decServer.(*model.Server)
	}

	var content [][]string
	content = append(content, []string{"Title", "IP", "Username", "Password", "URL", "HostingUserName",
		"HostingPassword", "AdminUsername", "AdminPassword", "Extra"})
	for i := range serverList {
		content = append(content, []string{serverList[i].Title,
			serverList[i].IP, serverList[i].Username,
			serverList[i].Password, serverList[i].URL,
			serverList[i].HostingUsername, serverList[i].HostingPassword,
			serverList[i].AdminUsername, serverList[i].AdminPassword,
			serverList[i].Extra})
	}

	return content
}

func generateZip(csvFiles []csvFile) ([]byte, error) {
	// Create a buffer to write our archive to.
	buf := new(bytes.Buffer)

	// Create a new zip archive.
	w := zip.NewWriter(buf)

	for _, csvFile := range csvFiles {
		f, err := w.Create(csvFile.Name)
		if err != nil {
			return nil, err
		}
		_, err = f.Write(csvFile.Data)
		if err != nil {
			return nil, err
		}
	}

	// Make sure to check the error on Close.
	err := w.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func generateCVS(csvModels []csvModel) ([]csvFile, error) {
	var files []csvFile

	for _, data := range csvModels {
		b := &bytes.Buffer{} // creates IO Writer
		csvWriter := csv.NewWriter(b)
		err := csvWriter.WriteAll(data.Data)
		if err != nil {
			return nil, err
		}
		csvWriter.Flush()

		// Create file object
		file := csvFile{
			Name: data.Name + ".csv",
			Data: b.Bytes(),
		}

		files = append(files, file)
	}

	return files, nil
}

// csv model
type csvModel struct {
	Name string
	Data [][]string
}

// csvFile model
type csvFile struct {
	Name string
	Data []byte
}
