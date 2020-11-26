package app

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/viper"
)

func tearDown(pattern string) {
	files, err := filepath.Glob(pattern)
	if err != nil {
		panic(err)
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			panic(err)
		}
	}
}

func generateFakeData(v int) error {
	var err error
	for i := 0; i < v; i++ {
		_, err = ioutil.TempFile("/tmp", "passwall.*.bak")
		if err != nil {
			log.Fatal(err)
		}
	}
	return err
}

func TestGetBackupFiles(t *testing.T) {
	viper.Set("backup.folder", "/tmp")
	tests := []struct {
		name          string
		run           func(v int) error
		expectedCount int
	}{
		{name: "With one file", run: generateFakeData, expectedCount: 1},
		{name: "With five files", run: generateFakeData, expectedCount: 4},
		{name: "With fourteen files", run: generateFakeData, expectedCount: 9},
		{name: "With seventeen  files", run: generateFakeData, expectedCount: 3},
	}
	for _, tt := range tests {
		tearDown("/tmp/passwall.*.bak")
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(tt.expectedCount); err != nil {
				t.Errorf("Unable to create temp files %v", err)
			}
			got, err := GetBackupFiles()
			actualCount := len(got)
			if err != nil {
				t.Errorf("GetBackupFiles() error = %v", err)
				return
			}
			if tt.expectedCount != actualCount {
				t.Errorf("Expected number of files does not match excp: %d, got: %d", tt.expectedCount, actualCount)
				return
			}

		})

	}

}

func TestRotateBackup(t *testing.T) {

	tests := []struct {
		name        string
		backupFiles []os.FileInfo
		wantErr     bool
	}{
		{name: "NIL value for backup files ", backupFiles: nil, wantErr: true},
		{name: "Non-nil backup files", backupFiles: []os.FileInfo{}, wantErr: false},
	}
	for _, tt := range tests {
		tearDown("/tmp/passwall.*.bak")
		t.Run(tt.name, func(t *testing.T) {
			if err := rotateBackup(tt.backupFiles); (err != nil) != tt.wantErr {
				t.Errorf("rotateBackup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

}
