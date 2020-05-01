package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pass-wall/passwall-server/internal/common"
	"github.com/pass-wall/passwall-server/internal/encryption"
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// Restore restores logins from backup file ./store/passwall-{BACKUP_DATE}.bak
func Restore(w http.ResponseWriter, r *http.Request) {
	var restoreDTO model.RestoreDTO

	// get restoreDTO
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&restoreDTO); err != nil {
		common.RespondWithError(w, http.StatusUnprocessableEntity, "Invalid json provided")
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
		common.RespondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	loginsByte := encryption.DecryptFile(backupPath, viper.GetString("server.passphrase"))

	var loginDTOs []model.LoginDTO
	json.Unmarshal(loginsByte, &loginDTOs)

	db := storage.GetDB()
	for i := range loginDTOs {

		login := model.Login{
			URL:      loginDTOs[i].URL,
			Username: loginDTOs[i].Username,
			Password: base64.StdEncoding.EncodeToString(encryption.Encrypt(loginDTOs[i].Password, viper.GetString("server.passphrase"))),
		}

		db.Save(&login)
	}

	response := model.Response{"Success", "Restore from backup completed successfully!"}
	common.RespondWithJSON(w, http.StatusOK, response)
}
