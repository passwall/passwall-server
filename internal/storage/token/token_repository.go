package token

import (
	"time"

	"github.com/jinzhu/gorm"
	"github.com/pass-wall/passwall-server/model"
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

func (p *Repository) Any(uuid string) bool {

	var token model.Token

	if !p.db.Where("uuid = ?", uuid).First(&token).RecordNotFound() {
		return true
	}

	return false

}

func (p *Repository) Save(userid int, uid uuid.UUID, tkn string, expriydate time.Time) {

	token := &model.Token{
		UserId:     userid,
		UUID:       uid,
		Token:      tkn,
		ExpiryTime: expriydate,
	}
	p.db.Create(token)

}

func (p *Repository) Delete(userid int) {
	p.db.Delete(model.Token{}, "user_id = ?", userid)
}

func (p *Repository) DeleteByUUID(uuid string) {
	p.db.Delete(model.Token{}, "uuid = ?", uuid)
}

// Migrate ...
func (p *Repository) Migrate() error {
	return p.db.AutoMigrate(&model.Token{}).Error
}
