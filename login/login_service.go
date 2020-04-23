package login

// LoginService ...
type LoginService struct {
	LoginRepository LoginRepository
}

// NewLoginService ...
func NewLoginService(p LoginRepository) LoginService {
	return LoginService{LoginRepository: p}
}

// Find Same Passwords Logins ...
func (p *LoginService) FindSamePassword(password string) (Urls, error) {

	logins, err := p.LoginRepository.All()

	logins = DecryptLoginPasswords(logins)

	newUrls := Urls{}

	for _, login := range logins {
		if login.Password == password {
			newUrls.AddItem(Url{Name: login.URL})
		}
	}

	return newUrls, err

}

// FindAll ...
func (p *LoginService) FindAll(argsStr map[string]string, argsInt map[string]int) ([]Login, error) {
	return p.LoginRepository.FindAll(argsStr, argsInt)
}

// FindByID ...
func (p *LoginService) FindByID(id uint) (Login, error) {
	return p.LoginRepository.FindByID(id)
}

// Save ...
func (p *LoginService) Save(login Login) (Login, error) {
	return p.LoginRepository.Save(login)
}

// Delete ...
func (p *LoginService) Delete(id uint) error {
	return p.LoginRepository.Delete(id)
}

// Migrate ...
func (p *LoginService) Migrate() {
	p.LoginRepository.Migrate()
}
