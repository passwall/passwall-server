package util

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pass-wall/passwall-server/helper"
	"github.com/pass-wall/passwall-server/login"
	"github.com/pass-wall/passwall-server/pkg/database"
	"github.com/spf13/viper"
)

// TODO: This backup endpoiont can only be triggered manually
// There should be an extra option to trigger it with time by cron job

// Backup gets all logins, compresses with passphrase and saves to ./store
func Backup(c *gin.Context) {
	db := database.GetDB()

	var logins []login.Login
	db.Find(&logins)
	logins = login.DecryptLoginPasswords(logins)

	// Struct to []byte
	loginBytes := new(bytes.Buffer)
	json.NewEncoder(loginBytes).Encode(logins)

	// TODO: Backup folder location should be a config.yml variable
	helper.EncryptFile("./store/passwall.bak", loginBytes.Bytes(), viper.GetString("server.passphrase"))

	response := login.LoginResponse{"Success", "Backup completed successfully!"}
	c.JSON(http.StatusNotFound, response)
}
