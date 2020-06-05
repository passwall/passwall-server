package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pass-wall/passwall-server/internal/config"
	"github.com/pass-wall/passwall-server/internal/storage"
)

func TestHealthCheck(t *testing.T) {
	// create valid database config
	// should be same with the one on github actions

	mockDBConfig := &config.DatabaseConfiguration{
		Name:     "passwall",
		Username: "postgres",
		Password: "postgres",
		Host:     "localhost",
		Port:     "5432",
		LogMode:  false,
	}

	db, err := storage.New(mockDBConfig)

	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := HealthCheck(db)

	handler.ServeHTTP(rr, req)
	// more test cases could be added
	expected := `{"API":{"StatusCode":200,"Err":null},"Database":{"StatusCode":200,"Err":null}}`

	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}

}
