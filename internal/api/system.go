package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v2"

	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	"github.com/passwall/passwall-server/pkg/buildvars"
	"github.com/passwall/passwall-server/pkg/logger"
)

const (
	//InvalidJSON represents a message for invalid json
	InvalidJSON = "Invalid json provided"
	//RestoreBackupSuccess represents a message when restoring from backap successfully
	RestoreBackupSuccess = "Restore from backup completed successfully!"
	//ImportSuccess represents when inporting successgully
	ImportSuccess = "Import finished successfully!"
	//BackupSuccess represents when backup completed successfully
	BackupSuccess = "Backup completed successfully!"
)

// CheckUpdate generates new password
func CheckUpdate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	product := vars["product"]

	if product != "1" {
		RespondWithError(w, http.StatusNotFound, "Product not found")
		return
	}

	type Update struct {
		LatestVersion string `json:"latest_version"`
		DownloadURL   string `json:"download_url"`
		ProductURL    string `json:"product_url"`
	}

	update := Update{
		LatestVersion: buildvars.Version,
		DownloadURL:   "https://passwall.io/download/passwall-macos/",
		ProductURL:    "https://signup.passwall.io",
	}

	RespondWithJSON(w, http.StatusOK, update)
}

// Languages ...
func findLanguageFiles(folder string) ([]string, error) {
	items := []string{}

	files, err := ioutil.ReadDir(folder)
	if err != nil {
		return items, err
	}

	s := []string{}
	for _, f := range files {
		// Since Split function returns string slice first split filename from extension
		e := strings.Split(f.Name(), ".")
		// Than split from the language part eg localization-xx
		l := strings.Split(e[0], "-")
		s = append(s, l[1])
	}

	return items, nil
}

// Languages ...
func Languages(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type Languages struct {
			Item []string `json:"languages"`
		}

		langItems, err := findLanguageFiles("../../store")
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
		}

		langs := Languages{
			Item: langItems,
		}

		RespondWithJSON(w, http.StatusOK, langs.Item)
	}
}

// Language ...
func Language(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		lang := vars["lang"]

		if lang != "tr" && lang != "en" {
			RespondWithError(w, http.StatusNotFound, "Language not found")
			return
		}

		yamlFile, err := os.Open("./store/localization-" + lang + ".yml")
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer yamlFile.Close()

		byteValue, err := ioutil.ReadAll(yamlFile)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var langs model.Language
		err = yaml.Unmarshal(byteValue, &langs)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK, langs)
	}
}

// Import ...
func Import(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payloadList []model.LoginDTO

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payloadList); err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		defer r.Body.Close()

		for _, loginDTO := range payloadList {
			// Add new login to db
			schema := r.Context().Value("schema").(string)
			_, err := app.CreateLogin(s, &loginDTO, schema)
			if err != nil {
				RespondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		response := model.Response{
			Code:    http.StatusOK,
			Status:  "Success",
			Message: "Import completed successfully!",
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}

// Export exports all data as CSV file
func Export(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		type AllModels struct {
			Logins       []model.Login
			BankAccounts []model.BankAccount
			CreditCards  []model.CreditCard
			Emails       []model.Email
			Notes        []model.Note
			Servers      []model.Server
		}

		var allRecords AllModels

		schema := r.Context().Value("schema").(string)

		if l, err := app.FindAllLogins(s, schema); err != nil {
			logger.Errorf("Error while getting logins: %v", err)
		} else {
			allRecords.Logins = l
		}

		if ba, err := app.FindAllBankAccounts(s, schema); err != nil {
			logger.Errorf("Error while getting logins: %v", err)
		} else {
			allRecords.BankAccounts = ba
		}

		if cc, err := app.FindAllCreditCards(s, schema); err != nil {
			logger.Errorf("Error while getting logins: %v", err)
		} else {
			allRecords.CreditCards = cc
		}

		if nt, err := app.FindAllNotes(s, schema); err != nil {
			logger.Errorf("Error while getting logins: %v", err)
		} else {
			allRecords.Notes = nt
		}

		if sr, err := app.FindAllServers(s, schema); err != nil {
			logger.Errorf("Error while getting logins: %v", err)
		} else {
			allRecords.Servers = sr
		}

		if em, err := app.FindAllEmails(s, schema); err != nil {
			logger.Errorf("Error while getting logins: %v", err)
		} else {
			allRecords.Emails = em
		}

		RespondWithJSON(w, http.StatusOK, allRecords)
	}
}
