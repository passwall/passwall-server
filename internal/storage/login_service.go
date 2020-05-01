package storage

import "github.com/pass-wall/passwall-server/model"

// LoginService ...
type LoginService struct {
	LoginRepository LoginRepository
}

// NewLoginService ...
func NewLoginService(p LoginRepository) LoginService {
	return LoginService{LoginRepository: p}
}

// All ...
func (p *LoginService) All() ([]model.Login, error) {
	return p.LoginRepository.All()
}

// FindAll ...
func (p *LoginService) FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.Login, error) {
	return p.LoginRepository.FindAll(argsStr, argsInt)
}

// FindByID ...
func (p *LoginService) FindByID(id uint) (model.Login, error) {
	return p.LoginRepository.FindByID(id)
}

// Save ...
func (p *LoginService) Save(login model.Login) (model.Login, error) {
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
