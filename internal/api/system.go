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
	"github.com/pass-wall/passwall-server/internal/app"
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

const (
	InvalidJSON          = "Invalid json provided"
	RestoreBackupSuccess = "Restore from backup completed successfully!"
	ImportSuccess        = "Import finished successfully!"
	BackupSuccess        = "Backup completed successfully!"
)

// GeneratePassword generates new password
func CheckUpdate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	product := vars["product"]

	if product != "1" {
		RespondWithError(w, http.StatusNotFound, "Product not found")
		return
	}

	type Update struct {
		LatestVersion string `json:"latest_version"`
		DownloadUrl   string `json:"download_url"`
		ProductUrl    string `json:"product_url"`
	}

	update := Update{
		LatestVersion: "0.1.2",
		DownloadUrl:   "https://passwall.io/download/passwall-macos/",
		ProductUrl:    "https://signup.passwall.io",
	}

	RespondWithJSON(w, http.StatusOK, update)
}

// Languages ...
func Languages(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type Languages struct {
			Item []string `json:"languages"`
		}

		// TODO: Read store folder and parse of localization files for this slice
		langs := Languages{
			Item: []string{"tr", "en"},
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
/* func Import(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		url := r.FormValue("url")
		username := r.FormValue("username")
		password := r.FormValue("password")

		uploadedFile, err := upload(r)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer uploadedFile.Close()

		// Go to first line of file
		uploadedFile.Seek(0, 0)

		// Read file content and add logins to db
		err = app.InsertValues(s, url, username, password, uploadedFile)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Delete imported file
		err = os.Remove(uploadedFile.Name())
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		response := model.Response{http.StatusOK, Success, ImportSuccess}
		RespondWithJSON(w, http.StatusOK, response)
	}
} */

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

		loginsByte := app.DecryptFile(backupPath, viper.GetString("server.passphrase"))

		var loginDTOs []model.LoginDTO
		json.Unmarshal(loginsByte, &loginDTOs)
		schema := r.Context().Value("schema").(string)
		for i := range loginDTOs {

			login := &model.Login{
				URL:      loginDTOs[i].URL,
				Username: loginDTOs[i].Username,
				Password: base64.StdEncoding.EncodeToString(app.Encrypt(loginDTOs[i].Password, viper.GetString("server.passphrase"))),
			}

			s.Logins().Save(login, schema)
		}

		response := model.Response{http.StatusOK, Success, RestoreBackupSuccess}
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
