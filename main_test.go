package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/yakuter/gpass/model"
	"github.com/yakuter/gpass/pkg/router"
)

const JWT_TOKEN string = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2MTc3NzI1NzIsIm9yaWdfaWF0IjoxNTg2NjY4NTcyLCJ1c2VybmFtZSI6ImdwYXNzIn0.LD8UmRLHoWMY7RDVQsxtePPWeDXmjcxs9uwHJAwEhL4"

func TestGetMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Set this value for an existing login ID
	// var ID int = 3
	// IDStr := strconv.Itoa(ID)

	// Set this value for a non existing login ID
	var nonID int = 999
	nonIDStr := strconv.Itoa(nonID)

	// Setting variables
	var logins []model.Login
	// var loginModel model.Login
	var resultModel model.Result

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
				result := row.returnObject.([]model.Login)
				err = json.Unmarshal(body, &result)
				assert.Nil(err)
			case "Get Single Login":
				result := row.returnObject.(model.Login)
				err = json.Unmarshal(body, &result)
				assert.Nil(err)
			case "Get False Single Login":
				result := row.returnObject.(model.Result)
				err = json.Unmarshal(body, &result)
				assert.Nil(err)
			case "Get Wrong ID Format":
				result := row.returnObject.(model.Result)
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
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return req, w
}
