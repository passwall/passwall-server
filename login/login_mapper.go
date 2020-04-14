package login

import (
	"net/url"
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
	var loginURL string
	u, err := url.Parse(login.URL)
	if err != nil {
		loginURL = login.URL
	} else {
		loginURL = u.Host
	}

	return LoginDTO{
		ID:       login.ID,
		URL:      loginURL,
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
