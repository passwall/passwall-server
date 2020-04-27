package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/pass-wall/passwall-server/internal/encryption"
	"github.com/pass-wall/passwall-server/internal/store"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// Backup gets all logins, compresses with passphrase and saves to ./store
func Backup(w http.ResponseWriter, r *http.Request) {
	err := BackupData()

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := model.Response{"Success", "Backup completed successfully!"}
	respondWithJSON(w, http.StatusOK, response)
}

// BackupData ...
func BackupData() error {
	backupFolder := viper.GetString("backup.folder")
	backupPath := fmt.Sprintf("%s/passwall.bak", backupFolder)

	db := store.GetDB()

	var logins []model.Login
	db.Find(&logins)
	logins = DecryptLoginPasswords(logins)

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
