package login

// LoginService ...
type LoginService struct {
	LoginRepository LoginRepository
}

// NewLoginService ...
func NewLoginService(p LoginRepository) LoginService {
	return LoginService{LoginRepository: p}
}

// FindAll ...
func (p *LoginService) FindAll() ([]Login, error) {
	return p.LoginRepository.FindAll()
}

// Search ...
func (p *LoginService) Search(keyword string) ([]Login, error) {
	return p.LoginRepository.Search(keyword)
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
