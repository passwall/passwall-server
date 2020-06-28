package login

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-test/deep"
	"github.com/jinzhu/gorm"
	"github.com/pass-wall/passwall-server/model"
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

	// Initialize repository
	loginRepository := NewRepository(mockDB)

	const sqlSelectAll = `SELECT * FROM "user-test"."logins"  WHERE "user-test"."logins"."deleted_at" IS NULL`
	mock.ExpectQuery(regexp.QuoteMeta(sqlSelectAll)).
		WillReturnRows(sqlmock.NewRows(nil))

	expected := []model.Login{}
	loginList, err := loginRepository.All("user-test")
	assert.Nil(t, err)

	assert.Nil(t, deep.Equal(expected, loginList))
}

func TestFindByID(t *testing.T) {

	// Create mock db
	mockDB, mock := dbSetup()

	// Initialize repository
	loginRepository := NewRepository(mockDB)

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

	rows := sqlmock.
		NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "title", "url", "username", "password"}).
		AddRow(login.ID, login.CreatedAt, login.UpdatedAt, login.DeletedAt, login.Title, login.URL, login.Username, login.Password)

	const sqlSelectOne = `SELECT * FROM "user-test"."logins" WHERE "user-test"."logins"."deleted_at" IS NULL AND ((id = $1))`

	mock.ExpectQuery(regexp.QuoteMeta(sqlSelectOne)).
		WithArgs(login.ID).
		WillReturnRows(rows)

	resultLogin, err := loginRepository.FindByID(login.ID, "user-test")
	assert.Nil(t, err)

	assert.Nil(t, deep.Equal(login, resultLogin))
}
