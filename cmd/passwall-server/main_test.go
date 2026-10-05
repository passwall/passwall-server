package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMigrationCommand(t *testing.T) {
	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })

	tests := []struct {
		args        []string
		isMigration bool
		runSeed     bool
	}{
		{args: []string{"passwall-server"}},
		{args: []string{"passwall-server", "serve"}},
		{args: []string{"passwall-server", "migrate"}, isMigration: true},
		{args: []string{"passwall-server", "migrate-and-seed"}, isMigration: true, runSeed: true},
	}

	for _, test := range tests {
		os.Args = test.args
		isMigration, runSeed := migrationCommand()
		assert.Equal(t, test.isMigration, isMigration)
		assert.Equal(t, test.runSeed, runSeed)
	}
}
