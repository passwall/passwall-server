package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/pass-wall/passwall-server/helper"
	"github.com/pass-wall/passwall-server/internal/database"
	"github.com/pass-wall/passwall-server/login"
	"github.com/spf13/viper"
)

// Restore restores logins from backup file ./store/passwall.bak
func Restore(c *gin.Context) {

	backupFolder := viper.GetString("backup.folder")
	backupPath := fmt.Sprintf("%s/passwall.bak", backupFolder)

	_, err := os.Open(backupPath)
	if err != nil {
		response := login.LoginResponse{"Error", fmt.Sprintf("Couldn't find backup file passwall.bak in %s folder", backupFolder)}
		c.JSON(http.StatusNotFound, response)
		return
	}

	loginsByte := helper.DecryptFile(backupPath, viper.GetString("server.passphrase"))

	var loginDTOs []login.LoginDTO
	json.Unmarshal(loginsByte, &loginDTOs)

	db := database.GetDB()
	for i := range loginDTOs {

		login := login.Login{
			URL:      loginDTOs[i].URL,
			Username: loginDTOs[i].Username,
			Password: base64.StdEncoding.EncodeToString(helper.Encrypt(loginDTOs[i].Password, viper.GetString("server.passphrase"))),
		}

		db.Save(&login)
	}

	response := login.LoginResponse{"Success", "Restore from backup completed successfully!"}
	c.JSON(http.StatusOK, response)
}
