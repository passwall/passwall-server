package login

import (
	"database/sql/driver"
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

	DB.LogMode(true)

	return DB, mock
}

func addDummyData() (*model.Login, *sqlmock.Rows) {
	login := &model.Login{
		ID:        1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		DeletedAt: nil,
		Title:     "Dummy Title",
		URL:       "http://dummy.com",
		Username:  "dummyuser",
		Password:  "dummypassword",
		Extra:     "dummy extra text",
	}

	return login, sqlmock.
		NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "title", "url", "username", "password", "extra"}).
		AddRow(login.ID, login.CreatedAt, login.UpdatedAt, login.DeletedAt, login.Title, login.URL, login.Username, login.Password, login.Extra)
}

func TestAll(t *testing.T) {

	// Create mock db
	mockDB, mock := dbSetup()

	// Initialize repository
	loginRepository := NewRepository(mockDB)

	login, rows := addDummyData()

	const sqlSelectAll = `SELECT * FROM "user-test"."logins"  WHERE "user-test"."logins"."deleted_at" IS NULL`
	mock.ExpectQuery(regexp.QuoteMeta(sqlSelectAll)).
		WillReturnRows(rows)

	expected := []model.Login{}
	expected = append(expected, *login)

	loginList, err := loginRepository.All("user-test")
	assert.Nil(t, err)

	assert.Nil(t, deep.Equal(expected, loginList))
}

func TestFindByID(t *testing.T) {

	// Create mock db
	mockDB, mock := dbSetup()

	// Initialize repository
	loginRepository := NewRepository(mockDB)

	login, rows := addDummyData()

	const sqlSelectOne = `SELECT * FROM "user-test"."logins" WHERE "user-test"."logins"."deleted_at" IS NULL AND ((id = $1))`

	mock.ExpectQuery(regexp.QuoteMeta(sqlSelectOne)).
		WithArgs(login.ID).
		WillReturnRows(rows)

	resultLogin, err := loginRepository.FindByID(login.ID, "user-test")
	assert.Nil(t, err)

	assert.Nil(t, deep.Equal(login, resultLogin))
}

func TestSave(t *testing.T) {

	// Create mock db
	mockDB, mock := dbSetup()

	// Initialize repository
	loginRepository := NewRepository(mockDB)

	login := &model.Login{
		Title:    "Dummy Title",
		URL:      "http://dummy.com",
		Username: "dummyuser",
		Password: "dummypassword",
		Extra:    "dummy extra text",
	}

	const sqlInsert = `INSERT INTO "user-test"."logins" ("created_at","updated_at","deleted_at","title","url","username","password","extra") VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING "user-test"."logins"."id"`

	mock.ExpectBegin() // start transaction
	mock.ExpectQuery(regexp.QuoteMeta(sqlInsert)).
		WithArgs(AnyTime{}, AnyTime{}, nil, login.Title, login.URL, login.Username, login.Password, login.Extra).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(login.ID))
	mock.ExpectCommit() // commit transaction

	resultLogin, err := loginRepository.Save(login, "user-test")
	assert.Nil(t, err)

	assert.Nil(t, deep.Equal(login, resultLogin))

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

type AnyTime struct{}

func (a AnyTime) Match(v driver.Value) bool {
	_, ok := v.(time.Time)
	return ok
}
