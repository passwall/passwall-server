package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pass-wall/passwall-server/internal/common"
	"github.com/pass-wall/passwall-server/internal/encryption"
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

const (
	timeFormat    = "2006-01-02T15-04-05"
	BackupSuccess = "Backup completed successfully!"
	Success       = "Success"
)

var (
	LoginEncodeError = errors.New("failed to encode login information")
	BackupError      = errors.New("error occurred while backing up data")
)

// Backup gets all logins, compresses with passphrase and saves to ./store
func Backup(w http.ResponseWriter, r *http.Request) {
	err := BackupData()

	if err != nil {
		common.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := model.Response{Code: http.StatusOK, Status: Success, Message: BackupSuccess}
	common.RespondWithJSON(w, http.StatusOK, response)
}

// List all backups
func ListBackup(w http.ResponseWriter, r *http.Request) {
	backupFiles, err := getBackupFiles()

	if err != nil {
		common.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var response []model.Backup
	for _, backupFile := range backupFiles {
		response = append(response, model.Backup{Name: backupFile.Name(), CreatedAt: backupFile.ModTime()})
	}

	common.RespondWithJSON(w, http.StatusOK, response)
}

// BackupData ...
func BackupData() error {
	backupFolder := viper.GetString("backup.folder")
	backupPath := fmt.Sprintf("%s/passwall-%s.bak", backupFolder, time.Now().Format(timeFormat))

	db := storage.GetDB()

	var loginList []model.Login
	db.Find(&loginList)
	loginList = DecryptLoginPasswords(loginList)

	// Struct to []byte vs. vs.
	loginBytes := new(bytes.Buffer)
	if err := json.NewEncoder(loginBytes).Encode(loginList); err != nil {
		return LoginEncodeError
	}

	if _, err := os.Stat(backupFolder); os.IsNotExist(err) {
		//http://permissions-calculator.org/
		//0755 Commonly used on web servers. The owner can read, write, execute.
		//Everyone else can read and execute but not modify the file.
		os.Mkdir(backupFolder, 0755)
	} else if err == nil {
		// is exist folder
	} else {
		err := BackupError
		return err
	}

	encryption.EncryptFile(backupPath, loginBytes.Bytes(), viper.GetString("server.passphrase"))

	if err := rotateBackup(); err != nil {
		return err
	}

	return nil
}

// Rotate backup files
func rotateBackup() error {
	backupRotation := viper.GetInt("backup.rotation")
	backupFolder := viper.GetString("backup.folder")

	backupFiles, err := getBackupFiles()
	if err != nil {
		return err
	}

	if len(backupFiles) > backupRotation {
		sort.SliceStable(backupFiles, func(i, j int) bool {
			return backupFiles[i].ModTime().After(backupFiles[j].ModTime())
		})

		for _, file := range backupFiles[backupRotation:] {
			_ = os.Remove(filepath.Join(backupFolder, file.Name()))
		}
	}

	return nil
}

func getBackupFiles() ([]os.FileInfo, error) {
	backupFolder := viper.GetString("backup.folder")

	files, err := ioutil.ReadDir(backupFolder)
	if err != nil {
		return nil, err
	}

	var backupFiles []os.FileInfo
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "passwall") && strings.HasSuffix(file.Name(), ".bak") {
			backupFiles = append(backupFiles, file)
		}
	}

	return backupFiles, nil
}
