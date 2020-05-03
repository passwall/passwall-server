package app

import (
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
)

// FindSamePassword ...
func FindSamePassword(s storage.Store, password model.Password) (model.URLs, error) {

	logins, err := s.Logins().All()
	if err != nil {
		return *new(model.URLs), nil
	}
	logins = DecryptLoginPasswords(logins)
	newUrls := model.URLs{Items: []string{}}

	for _, login := range logins {
		if login.Password == password.Password {
			newUrls.AddItem(login.URL)
		}
	}

	return newUrls, err
}
