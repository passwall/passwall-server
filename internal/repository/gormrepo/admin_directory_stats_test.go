package gormrepo

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestDirectorySchemaCountable(t *testing.T) {
	t.Parallel()

	assert.True(t, directorySchemaCountable("user_a1"))
	assert.False(t, directorySchemaCountable(""))
	assert.False(t, directorySchemaCountable("public"))
	assert.False(t, directorySchemaCountable("User-Schema"))
	assert.False(t, directorySchemaCountable("pg_temp"))
}

func TestIsMissingRelation(t *testing.T) {
	t.Parallel()

	assert.True(t, isMissingRelation(&pgconn.PgError{Code: "42P01"}))
	assert.True(t, isMissingRelation(&pgconn.PgError{Code: "3F000"}))
	assert.False(t, isMissingRelation(&pgconn.PgError{Code: "42501"}))
	assert.False(t, isMissingRelation(errors.New("connection refused")))
}
