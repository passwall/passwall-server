package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDatabasePinger struct {
	err   error
	calls int
}

func (f *fakeDatabasePinger) Ping(context.Context) error {
	f.calls++
	return f.err
}

func TestHealthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("liveness does not depend on readiness or database", func(t *testing.T) {
		db := &fakeDatabasePinger{err: errors.New("database unavailable")}
		router := gin.New()
		registerHealthRoutes(router, db, func() bool { return false })

		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))

		require.Equal(t, http.StatusOK, response.Code)
		assert.JSONEq(t, `{"status":"ok"}`, response.Body.String())
		assert.Equal(t, "no-store", response.Header().Get("Cache-Control"))
		assert.Zero(t, db.calls)
	})

	t.Run("readiness is disabled during startup or shutdown", func(t *testing.T) {
		db := &fakeDatabasePinger{}
		router := gin.New()
		registerHealthRoutes(router, db, func() bool { return false })

		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ready", nil))

		require.Equal(t, http.StatusServiceUnavailable, response.Code)
		assert.Zero(t, db.calls)
	})

	t.Run("readiness fails when database ping fails", func(t *testing.T) {
		db := &fakeDatabasePinger{err: errors.New("database unavailable")}
		router := gin.New()
		registerHealthRoutes(router, db, func() bool { return true })

		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ready", nil))

		require.Equal(t, http.StatusServiceUnavailable, response.Code)
		assert.Equal(t, 1, db.calls)
	})

	t.Run("readiness succeeds when app and database are ready", func(t *testing.T) {
		db := &fakeDatabasePinger{}
		router := gin.New()
		registerHealthRoutes(router, db, func() bool { return true })

		firstResponse := httptest.NewRecorder()
		router.ServeHTTP(firstResponse, httptest.NewRequest(http.MethodGet, "/ready", nil))
		secondResponse := httptest.NewRecorder()
		router.ServeHTTP(secondResponse, httptest.NewRequest(http.MethodGet, "/ready", nil))

		require.Equal(t, http.StatusOK, firstResponse.Code)
		assert.JSONEq(t, `{"status":"ready"}`, firstResponse.Body.String())
		require.Equal(t, http.StatusOK, secondResponse.Code)
		assert.Equal(t, 1, db.calls)
	})
}
