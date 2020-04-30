package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/pass-wall/passwall-server/internal/encryption"
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// Restore restores logins from backup file ./store/passwall.bak
func Restore(w http.ResponseWriter, r *http.Request) {

	backupFolder := viper.GetString("backup.folder")
	backupPath := fmt.Sprintf("%s/passwall.bak", backupFolder)

	_, err := os.Open(backupPath)
	if err != nil {
		respondWithError(w, http.StatusNotFound, err.Error())
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
	respondWithJSON(w, http.StatusOK, response)
}
