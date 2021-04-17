package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
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
	if mux.Vars(r)["product"] != "1" {
		RespondWithError(w, http.StatusNotFound, "Product not found")
		return
	}

	type Update struct {
		LatestVersion string `json:"latest_version"`
		DownloadURL   string `json:"download_url"`
		ProductURL    string `json:"product_url"`
	}

	RespondWithJSON(w, http.StatusOK,
		Update{
			LatestVersion: "0.1.4",
			DownloadURL:   "https://passwall.io/download/passwall-macos/",
			ProductURL:    "https://signup.passwall.io",
		})
}

// Languages ...
func findLanguageFiles(folder string) ([]string, error) {
	return make([]string, 0), nil
}

// Languages ...
func Languages(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		langItems, err := findLanguageFiles("../../store")
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
		}

		type Languages struct {
			Item []string `json:"languages"`
		}

		RespondWithJSON(w, http.StatusOK,
			Languages{
				Item: langItems,
			})
	}
}

// Language ...
func Language(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := mux.Vars(r)["lang"]

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

		if err := json.NewDecoder(r.Body).Decode(&payloadList); err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		defer r.Body.Close()

		// Add new login to db
		var loginDTO model.LoginDTO

		for i := range payloadList {
			// Decrypt payload
			if err := app.DecryptJSON(r.Context().Value("transmissionKey").(string), []byte(payloadList[i].Data), &loginDTO); err != nil {
				RespondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}

			_, err := app.CreateLogin(s, &loginDTO, r.Context().Value("schema").(string))
			if err != nil {
				RespondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		RespondWithJSON(w, http.StatusOK,
			model.Response{
				Code:    http.StatusOK,
				Status:  Success,
				Message: "Import completed successfully!",
			})
	}
}

// Export exports all logins as CSV file
/* func Export(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		var loginList []model.Login
		s.Find(&loginList)

		loginList = app.DecryptLoginPasswords(loginList)

		var content [][]string
		content = append(content, []string{"URL", "Username", "Password"})
		for i := range loginList {
			content = append(content, []string{loginList[i].URL, loginList[i].Username, loginList[i].Password})
		}

		b := &bytes.Buffer{} // creates IO Writer
		csvWriter := csv.NewWriter(b)
		strWrite := content
		csvWriter.WriteAll(strWrite)
		csvWriter.Flush()

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment;filename=PassWall.csv")
		w.Write(b.Bytes())
	}
} */

// Restore restores logins from backup file ./store/passwall-{BACKUP_DATE}.bak
func Restore(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var restoreDTO model.RestoreDTO

		// get restoreDTO
		if err := json.NewDecoder(r.Body).Decode(&restoreDTO); err != nil {
			RespondWithError(w, http.StatusUnprocessableEntity, InvalidJSON)
			return
		}
		defer r.Body.Close()

		backupFile := restoreDTO.Name
		// add extension if there is no
		if len(filepath.Ext(restoreDTO.Name)) <= 0 {
			backupFile = fmt.Sprintf("%s%s", restoreDTO.Name, ".bak")
		}
		backupPath := filepath.Join(viper.GetString("backup.folder"), backupFile)

		_, err := os.Open(backupPath)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		var loginDTOs []model.LoginDTO
		json.Unmarshal(app.DecryptFile(backupPath, viper.GetString("server.passphrase")), &loginDTOs)
		schema := r.Context().Value("schema").(string)
		for i := range loginDTOs {

			login := &model.Login{
				URL:      loginDTOs[i].URL,
				Username: loginDTOs[i].Username,
				Password: base64.StdEncoding.EncodeToString(app.Encrypt(loginDTOs[i].Password, viper.GetString("server.passphrase"))),
			}

			s.Logins().Save(login, schema)
		}

		RespondWithJSON(w, http.StatusOK,
			model.Response{
				Code:    http.StatusOK,
				Status:  Success,
				Message: RestoreBackupSuccess})
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
