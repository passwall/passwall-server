package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/pass-wall/passwall-server/internal/database"
	"github.com/pass-wall/passwall-server/internal/encryption"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// Backup gets all logins, compresses with passphrase and saves to ./store
func Backup(c *gin.Context) {
	err := BackupData()

	if err != nil {
		log.Println(err)
		response := model.LoginResponse{"Error", err.Error()}
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	response := model.LoginResponse{"Success", "Backup completed successfully!"}
	c.JSON(http.StatusOK, response)
}

// BackupData ...
func BackupData() error {
	backupFolder := viper.GetString("backup.folder")
	backupPath := fmt.Sprintf("%s/passwall.bak", backupFolder)

	db := database.GetDB()

	var logins []model.Login
	db.Find(&logins)
	logins = model.DecryptLoginPasswords(logins)

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

	encryption.EncryptFile(backupPath, loginBytes.Bytes(), viper.GetString("server.passphrase"))
	return nil
}
