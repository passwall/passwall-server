package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"github.com/spf13/viper"
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
		var payloadList []model.Payload
		var loginDTO model.LoginDTO

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payloadList); err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		defer r.Body.Close()

		for i := range payloadList {
			// Decrypt payload
			key := r.Context().Value("transmissionKey").(string)
			if err := app.DecryptJSON(key, []byte(payloadList[i].Data), &loginDTO); err != nil {
				RespondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}

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
		// Get all data from db
		logins := getLogins(s, w, r)
		bankAccounts := getBankAccounts(s, w, r)
		creditCards := getCreditCards(s, w, r)
		emails := getEmails(s, w, r)
		notes := getNotes(s, w, r)
		servers := getServers(s, w, r)

		// Create data array
		var csvModels []csvModel

		// Append all data to array
		csvModels = append(csvModels, csvModel{"Logins", logins}, csvModel{"BankAccounts", bankAccounts},
			csvModel{"CreditCards", creditCards}, csvModel{"Emails", emails},
			csvModel{"Notes", notes}, csvModel{"Servers", servers})

		// Generate csv files from csv data
		csvFiles, err := generateCVS(csvModels)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Generate zip file from csv files
		zipFile, err := generateZip(csvFiles)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", "attachment;filename=PassWall.zip")
		w.Write(zipFile)
	}
}

// Restore restores logins from backup file ./store/passwall-{BACKUP_DATE}.bak
func Restore(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var restoreDTO model.RestoreDTO

		// get restoreDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&restoreDTO); err != nil {
			RespondWithError(w, http.StatusUnprocessableEntity, InvalidJSON)
			return
		}
		defer r.Body.Close()

		backupFolder := viper.GetString("backup.folder")
		backupFile := restoreDTO.Name
		// add extension if there is no
		if len(filepath.Ext(restoreDTO.Name)) <= 0 {
			backupFile = fmt.Sprintf("%s%s", restoreDTO.Name, ".bak")
		}
		backupPath := filepath.Join(backupFolder, backupFile)

		_, err := os.Open(backupPath)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		loginsByte, err := app.DecryptFile(backupPath, viper.GetString("server.passphrase"))
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var loginDTOs []model.LoginDTO
		json.Unmarshal(loginsByte, &loginDTOs)
		schema := r.Context().Value("schema").(string)
		for i := range loginDTOs {
			password, err := app.Encrypt(loginDTOs[i].Password, viper.GetString("server.passphrase"))
			if err != nil {
				logger.Errorf("Error while encrypting: %s", err.Error())
			}

			login := &model.Login{
				URL:      loginDTOs[i].URL,
				Username: loginDTOs[i].Username,
				Password: base64.StdEncoding.EncodeToString(password),
			}

			s.Logins().Update(login, schema)
		}

		response := model.Response{Code: http.StatusOK, Status: Success, Message: RestoreBackupSuccess}
		RespondWithJSON(w, http.StatusOK, response)
	}
}

// Backup backups the store
/* func Backup(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := app.BackupData(s)

		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		response := model.Response{http.StatusOK, Success, BackupSuccess}
		RespondWithJSON(w, http.StatusOK, response)
	}
} */

// ListBackup all backups
/* func ListBackup(w http.ResponseWriter, r *http.Request) {
	backupFiles, err := app.GetBackupFiles()

	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var response []model.Backup
	for _, backupFile := range backupFiles {
		response = append(response, model.Backup{Name: backupFile.Name(), CreatedAt: backupFile.ModTime()})
	}

	RespondWithJSON(w, http.StatusOK, response)
} */

func upload(r *http.Request) (*os.File, error) {

	// Max 10 MB
	r.ParseMultipartForm(10 << 20)

	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)

	if ext != ".csv" {
		return nil, fmt.Errorf("%s unsupported filetype", ext)
	}

	tempFile, err := ioutil.TempFile("/tmp", "passwall-import-*.csv")
	if err != nil {
		return nil, err
	}

	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	tempFile.Write(fileBytes)

	return tempFile, err
}
