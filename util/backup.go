package util

import (
	"bytes"
	"encoding/json"
	"errors"
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

// Backup gets all logins, compresses with passphrase and saves to ./store
func Backup(c *gin.Context) {
	err := BackupData()

	if err != nil {
		log.Println(err)
		response := login.LoginResponse{"Error", err.Error()}
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	response := login.LoginResponse{"Success", "Backup completed successfully!"}
	c.JSON(http.StatusOK, response)
}

// BackupData ...
func BackupData() error {
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
		err := errors.New("Error occured while backuping data")
		return err
	}

	helper.EncryptFile(backupPath, loginBytes.Bytes(), viper.GetString("server.passphrase"))
	return nil
}
