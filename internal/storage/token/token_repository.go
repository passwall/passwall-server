package token

import (
	"errors"
	"time"

	"github.com/passwall/passwall-server/model"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
)

// Repository ...
type Repository struct {
	db *gorm.DB
}

// NewRepository ...
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

//Any represents any match
func (p *Repository) FindByUUID(uuid string) (model.Token, error) {
	token := model.Token{}
	err := p.db.Where("uuid = ?", uuid).First(&token).Error
	return token, err
}

// Create creates model to database
func (p *Repository) Create(userid int, uid uuid.UUID, tkn string, expriydate time.Time, transmissionKey string) {

	token := &model.Token{
		UserID:          userid,
		UUID:            uid,
		Token:           tkn,
		ExpiryTime:      expriydate,
		TransmissionKey: transmissionKey,
	}
	p.db.Create(token)

}

//Delete deletes from database
func (p *Repository) Delete(userid int) {
	p.db.Delete(model.Token{}, "user_id = ?", userid)
}

//DeleteByUUID deletes from database by uuid
func (p *Repository) DeleteByUUID(uuid string) {
	p.db.Delete(model.Token{}, "uuid = ?", uuid)
}

// Migrate ...
func (p *Repository) Migrate() error {
	return p.db.AutoMigrate(&model.Token{})
}
