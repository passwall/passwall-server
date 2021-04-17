package token

import (
	"time"

	"github.com/jinzhu/gorm"
	"github.com/passwall/passwall-server/model"
	uuid "github.com/satori/go.uuid"
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
func (p *Repository) Any(uuid string) (model.Token, bool) {
	token := model.Token{}

	if !p.db.Where("uuid = ?", uuid).First(&token).RecordNotFound() {
		return token, true
	}

	return token, false
}

//Save saves model to database
func (p *Repository) Save(userid int, uid uuid.UUID, tkn string, expriydate time.Time, transmissionKey string) {
	p.db.Create(
		&model.Token{
			UserID:          userid,
			UUID:            uid,
			Token:           tkn,
			ExpiryTime:      expriydate,
			TransmissionKey: transmissionKey,
		})
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
	return p.db.AutoMigrate(&model.Token{}).Error
}
