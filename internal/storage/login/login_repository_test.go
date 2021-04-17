package login

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-test/deep"
	"github.com/jinzhu/gorm"
	"github.com/passwall/passwall-server/model"
	"github.com/stretchr/testify/assert"
)

func dbSetup() (*gorm.DB, sqlmock.Sqlmock) {
	db, mock, _ := sqlmock.New()
	DB, _ := gorm.Open("postgres", db)

	DB.LogMode(false)

	return DB, mock
}

func TestAll(t *testing.T) {
	// Create mock db
	mockDB, mock := dbSetup()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user-test"."logins"  WHERE "user-test"."logins"."deleted_at" IS NULL`)).
		WillReturnRows(sqlmock.NewRows(nil))

	loginList, err := NewRepository(mockDB).All("user-test")
	assert.Nil(t, err)

	assert.Nil(t, deep.Equal([]model.Login{}, loginList))
}

func TestFindByID(t *testing.T) {
	// Create mock db
	mockDB, mock := dbSetup()

	login := &model.Login{
		ID:        1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		DeletedAt: nil,
		Title:     "Dummy Title",
		URL:       "http://dummy.com",
		Username:  "dummyuser",
		Password:  "dummypassword",
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user-test"."logins" WHERE "user-test"."logins"."deleted_at" IS NULL AND ((id = $1))`)).
		WithArgs(login.ID).
		WillReturnRows(
			sqlmock.
				NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "title", "url", "username", "password"}).
				AddRow(login.ID, login.CreatedAt, login.UpdatedAt, login.DeletedAt, login.Title, login.URL, login.Username, login.Password))

	resultLogin, err := NewRepository(mockDB).FindByID(login.ID, "user-test")
	assert.Nil(t, err)

	assert.Nil(t, deep.Equal(login, resultLogin))
}
