package login

import (
	"strings"
)

// ToLogin ...
func ToLogin(loginDTO LoginDTO) Login {
	return Login{
		URL:      loginDTO.URL,
		Username: loginDTO.Username,
		Password: loginDTO.Password,
	}
}

// ToLoginDTO ...
func ToLoginDTO(login Login) LoginDTO {

	trims := []string{"https://", "http://", "www"}
	for i := range trims {
		login.URL = strings.TrimPrefix(login.URL, trims[i])
	}

	return LoginDTO{
		ID:       login.ID,
		URL:      login.URL,
		Username: login.Username,
		Password: login.Password,
	}
}

// ToLoginDTOs ...
func ToLoginDTOs(logins []Login) []LoginDTO {
	loginDTOs := make([]LoginDTO, len(logins))

	for i, itm := range logins {
		loginDTOs[i] = ToLoginDTO(itm)
	}

	return loginDTOs
}
