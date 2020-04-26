package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/pass-wall/passwall-server/login"
	"github.com/pass-wall/passwall-server/internal/router"
	"github.com/stretchr/testify/assert"
)

const JWT_TOKEN string = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE1OTYwNTM0NDQsIm9yaWdfaWF0IjoxNTg3NDEzNDQ0LCJ1c2VybmFtZSI6InBhc3N3YWxsIn0.RMF3UqzoZRzYLjvRdoHcqwXAfJHZeD2xC0n2q_1pHno"

// TODO: The tests here are not suitable.
// There should be SQL mocking with gorm driver object
// https://github.com/Selvatico/go-mocket seems like a good packet for this job

// using :memory: instead file could be an option for testing
// https://gorm.io/docs/connecting_to_the_database.html#Sqlite3

func TestGetMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Set this value for an existing login ID
	// var ID int = 1
	// IDStr := strconv.Itoa(ID)

	// Set this value for a non existing login ID
	var nonID int = 999
	nonIDStr := strconv.Itoa(nonID)

	// Setting variables
	var logins []login.Login
	// var loginModel login.Login
	var resultModel login.LoginResponse

	// Creating test table
	var table = []struct {
		name         string
		method       string
		url          string
		statusCode   int
		returnObject interface{}
	}{
		{"GET All Logins", "GET", "/logins/", http.StatusOK, logins}, // 200
		// {"Get Single Login", "GET", "/logins/" + IDStr, http.StatusOK, loginModel},                 // 200
		{"Get False Single Login", "GET", "/logins/" + nonIDStr, http.StatusNotFound, resultModel}, // 404
		{"Get Wrong ID Format", "GET", "/logins/xxx", http.StatusBadRequest, resultModel},          // 400
	}

	var assert = assert.New(t)
	// Test loop for all table rows
	for _, row := range table {

		t.Run(row.name, func(t *testing.T) {
			req, w := makeGetRequest(row.method, row.url)

			assert.Equal(row.method, req.Method, "HTTP request method error")
			assert.Equal(row.statusCode, w.Code, "HTTP request status code error")

			body, err := ioutil.ReadAll(w.Body)
			assert.Nil(err)

			switch row.name {
			case "GET All Logins":
				result := row.returnObject.([]login.Login)
				err = json.Unmarshal(body, &result)
				assert.Nil(err)
			case "Get Single Login":
				result := row.returnObject.(login.Login)
				err = json.Unmarshal(body, &result)
				assert.Nil(err)
			case "Get False Single Login":
				result := row.returnObject.(login.LoginResponse)
				err = json.Unmarshal(body, &result)
				assert.Nil(err)
			case "Get Wrong ID Format":
				result := row.returnObject.(login.LoginResponse)
				err = json.Unmarshal(body, &result)
				assert.Nil(err)
			}
		})
	}
}

func makeGetRequest(method, url string) (*http.Request, *httptest.ResponseRecorder) {
	r := router.Setup()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, url, nil)
	req.Header.Add("Authorization", "Bearer "+JWT_TOKEN)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return req, w
}
