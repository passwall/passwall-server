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
func (p *LoginService) FindAll() []Login {
	return p.LoginRepository.FindAll()
}

// FindByID ...
func (p *LoginService) FindByID(id uint) Login {
	return p.LoginRepository.FindByID(id)
}

// Save ...
func (p *LoginService) Save(login Login) Login {
	return p.LoginRepository.Save(login)
}

// Delete ...
func (p *LoginService) Delete(id uint) {
	p.LoginRepository.Delete(id)
}

// Migrate ...
func (p *LoginService) Migrate() {
	p.LoginRepository.Migrate()
}
