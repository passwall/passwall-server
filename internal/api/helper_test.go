package api_test

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/passwall/passwall-server/internal/api"
)

func TestToBody(t *testing.T) {

	jsonstr := `{
		"id": 1,
		"title": "Google",
		"url": "https://google.com",
		"username": "yakuter",
		"password": "dell3625",
		"extra": "123456"
	}`

	env := "dev"
	transmissionKey := "dummykey"

	// Generate request body
	r := new(http.Request)
	r.Body = ioutil.NopCloser(strings.NewReader(jsonstr))

	rBefore := r.Body

	err := api.ToBody(r, env, transmissionKey)
	if err != nil {
		t.Errorf("ToBody Error: %s", err.Error())
	}

	rAfter := r.Body

	if env == "dev" && rBefore != rAfter {
		t.Error("Incoming and outgoing request body should be same on dev!")
	}

	if env == "prod" && rBefore == rAfter {
		t.Error("Incoming and outgoing request body shouldn't be same on prod!")
	}

}
