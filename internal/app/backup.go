package app

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

const (
	timeFormat = "2006-01-02T15-04-05"
)

var (
	errBackup           = errors.New("error occurred while backing up data")
	errNoBackupFilesErr = errors.New("no backup file  provided")
)

// BackupData ...
/* func BackupData(s storage.Store) error {
	backupFolder := viper.GetString("backup.folder")
	backupPath := fmt.Sprintf("%s/passwall-%s.bak", backupFolder, time.Now().Format(timeFormat))

	var loginList []model.Login
	s.Find(&loginList)
	loginList = DecryptLoginPasswords(loginList)

	// Struct to []byte vs. vs.
	loginBytes := new(bytes.Buffer)
	json.NewEncoder(loginBytes).Encode(loginList)

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

	EncryptFile(backupPath, loginBytes.Bytes(), viper.GetString("server.passphrase"))

	rotateBackup()

	return nil
} */

// Rotate backup files
func rotateBackup(backupFiles []os.FileInfo) error {
	backupRotation := viper.GetInt("backup.rotation")

	if backupFiles == nil {
		return errNoBackupFilesErr
	}

	if len(backupFiles) > backupRotation {
		sort.SliceStable(backupFiles, func(i, j int) bool {
			return backupFiles[i].ModTime().After(backupFiles[j].ModTime())
		})

		for _, file := range backupFiles[backupRotation:] {
			_ = os.Remove(filepath.Join(viper.GetString("backup.folder"), file.Name()))
		}
	}
	return nil
}

//GetBackupFiles retrieves backup files
func GetBackupFiles() ([]os.FileInfo, error) {
	files, err := ioutil.ReadDir(viper.GetString("backup.folder"))
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
