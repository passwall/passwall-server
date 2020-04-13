package login

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
