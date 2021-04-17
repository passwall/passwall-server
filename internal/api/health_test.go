package api

import (
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/storage"
)

func TestHealthCheck(t *testing.T) {
	// create valid database config
	// should be same with the one on github actions

	mockDB, err := storage.DBConn(
		&config.DatabaseConfiguration{
			Name:     "passwall",
			Username: "postgres",
			Password: "postgres",
			Host:     "localhost",
			Port:     "5432",
			LogMode:  false,
		})

	if err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	HealthCheck(storage.New(mockDB)).ServeHTTP(rr, req)
	// more test cases could be added
	expected := `{"api":{"status_code":200,"error":null},"database":{"status_code":200,"error":null}}`

	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}
