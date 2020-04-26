package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/pass-wall/passwall-server/internal/database"
	"github.com/pass-wall/passwall-server/internal/encryption"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// Restore restores logins from backup file ./store/passwall.bak
func Restore(c *gin.Context) {

	backupFolder := viper.GetString("backup.folder")
	backupPath := fmt.Sprintf("%s/passwall.bak", backupFolder)

	_, err := os.Open(backupPath)
	if err != nil {
		response := model.LoginResponse{"Error", fmt.Sprintf("Couldn't find backup file passwall.bak in %s folder", backupFolder)}
		c.JSON(http.StatusNotFound, response)
		return
	}

	loginsByte := encryption.DecryptFile(backupPath, viper.GetString("server.passphrase"))

	var loginDTOs []model.LoginDTO
	json.Unmarshal(loginsByte, &loginDTOs)

	db := database.GetDB()
	for i := range loginDTOs {

		login := model.Login{
			URL:      loginDTOs[i].URL,
			Username: loginDTOs[i].Username,
			Password: base64.StdEncoding.EncodeToString(encryption.Encrypt(loginDTOs[i].Password, viper.GetString("server.passphrase"))),
		}

		db.Save(&login)
	}

	response := model.LoginResponse{"Success", "Restore from backup completed successfully!"}
	c.JSON(http.StatusOK, response)
}
