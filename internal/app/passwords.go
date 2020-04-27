package app

import (
	"encoding/base64"
	"net/http"

	"github.com/pass-wall/passwall-server/internal/encryption"
	"github.com/pass-wall/passwall-server/internal/store"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// GeneratePassword generates new password
func GeneratePassword(w http.ResponseWriter, r *http.Request) {
	password := encryption.Password()
	response := model.Response{"Success", password}
	respondWithJSON(w, http.StatusOK, response)
}

// Find Same Passwords Logins ...
func FindSamePassword(p *store.LoginService, password model.Password) (model.URLs, error) {

	logins, err := p.LoginRepository.All()

	logins = DecryptLoginPasswords(logins)

	newUrls := model.URLs{Items: []string{}}

	for _, login := range logins {
		if login.Password == password.Password {
			newUrls.AddItem(login.URL)
		}
	}

	return newUrls, err

}

// DecryptLoginPasswords ...
func DecryptLoginPasswords(logins []model.Login) []model.Login {
	for i := range logins {
		if logins[i].Password == "" {
			continue
		}
		passByte, _ := base64.StdEncoding.DecodeString(logins[i].Password)
		passB64 := string(encryption.Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))
		logins[i].Password = passB64
	}
	return logins
}
