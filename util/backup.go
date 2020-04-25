package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

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

	backupFolder := viper.GetString("backup.folder")
	backupPath := fmt.Sprintf("%s/passwall.bak", backupFolder)

	db := database.GetDB()

	var logins []login.Login
	db.Find(&logins)
	logins = login.DecryptLoginPasswords(logins)

	// Struct to []byte
	loginBytes := new(bytes.Buffer)
	json.NewEncoder(loginBytes).Encode(logins)

	if _, err := os.Stat(backupFolder); os.IsNotExist(err) {
		//http://permissions-calculator.org/
		//0755 Commonly used on web servers. The owner can read, write, execute.
		//Everyone else can read and execute but not modify the file.
		os.Mkdir(backupFolder, 0755)
	} else if err == nil {
		// is exist folder
	} else {

		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"Status":  "Error",
			"Message": err.Error(),
		})

		return
	}

	helper.EncryptFile(backupPath, loginBytes.Bytes(), viper.GetString("server.passphrase"))

	response := login.LoginResponse{"Success", "Backup completed successfully!"}
	c.JSON(http.StatusOK, response)
}
