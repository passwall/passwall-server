package api

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-test/deep"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	"github.com/stretchr/testify/assert"
)

func TestFindAllLogins(t *testing.T) {
	w := httptest.NewRecorder()

	// Create mock db
	mockDB, mock := dbSetup()

	// Initialize router
	r := routersSetup(mockDB)

	// Generate dummy login
	var logins []model.Login
	login := model.Login{
		ID:        1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		DeletedAt: nil,
		Title:     "Dummy Title",
		URL:       "http://dummy.com",
		Username:  "dummyuser",
		Password:  "GRr4f5bWKolEVw8EjXSryNPvVLEorL3VILyYhMUkZiize6FlBvP4C1I=", // Encrypted "dummypassword"
	}
	logins = append(logins, login)

	// Add dummy login to dummy db table
	rows := sqlmock.
		NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "title", "url", "username", "password"}).
		AddRow(login.ID, login.CreatedAt, login.UpdatedAt, login.DeletedAt, login.Title, login.URL, login.Username, login.Password)

	// Define expected query
	const sqlSelectOne = `SELECT * FROM "user-test"."logins"`
	mock.ExpectQuery(regexp.QuoteMeta(sqlSelectOne)).
		WillReturnRows(rows)

	// Make request
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/logins", nil))

	// Check status code
	assert.Equal(t, http.StatusOK, w.Code, "Did not get expected HTTP status code, got")

	// Unmarshall response
	var resultLogins []model.Login
	decoder := json.NewDecoder(w.Body)
	if err := decoder.Decode(&resultLogins); err != nil {
		t.Error(err)
	}

	// Compare response and table data
	assert.Nil(t, deep.Equal(logins, resultLogins))

	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}

}

func TestFindLoginsByID(t *testing.T) {
	w := httptest.NewRecorder()

	// Create mock db
	mockDB, mock := dbSetup()

	// Initialize router
	r := routersSetup(mockDB)

	// Generate dummy login
	loginDTO := &model.LoginDTO{
		ID:       1,
		Title:    "Dummy Title",
		URL:      "http://dummy.com",
		Username: "dummyuser",
		Password: "dummypassword",
	}

	encryptedPassword := "GRr4f5bWKolEVw8EjXSryNPvVLEorL3VILyYhMUkZiize6FlBvP4C1I="

	// Add dummy login to dummy db table
	rows := sqlmock.
		NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "title", "url", "username", "password"}).
		AddRow(loginDTO.ID, time.Now(), time.Now(), nil, loginDTO.Title, loginDTO.URL, loginDTO.Username, encryptedPassword)

	// Define expected query
	const sqlSelectOne = `SELECT * FROM "user-test"."logins" WHERE "user-test"."logins"."deleted_at" IS NULL AND ((id = $1))`
	mock.ExpectQuery(regexp.QuoteMeta(sqlSelectOne)).
		WithArgs(loginDTO.ID).
		WillReturnRows(rows)

	// Make request
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/logins/1", nil))

	// Check status code
	assert.Equal(t, http.StatusOK, w.Code, "Did not get expected HTTP status code, got")

	// Unmarshall response
	resultLogin := new(model.LoginDTO)
	decoder := json.NewDecoder(w.Body)
	if err := decoder.Decode(&resultLogin); err != nil {
		t.Error(err)
	}

	// Compare response and table data
	assert.Nil(t, deep.Equal(loginDTO, resultLogin))

	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func dbSetup() (*gorm.DB, sqlmock.Sqlmock) {
	db, mock, _ := sqlmock.New()
	DB, _ := gorm.Open("postgres", db)
	// DB.LogMode(true)

	return DB, mock
}

func routersSetup(db *gorm.DB) *mux.Router {

	// Create storage with mock db
	store := storage.New(db)

	// Initialize router
	apiRouter := mux.NewRouter().PathPrefix("/api").Subrouter()

	// Login endpoints
	apiRouter.Handle("/logins", contextMiddleware(FindAllLogins(store))).Methods(http.MethodGet)
	apiRouter.Handle("/logins", contextMiddleware(CreateLogin(store))).Methods(http.MethodPost)
	apiRouter.Handle("/logins/{id:[0-9]+}", contextMiddleware(FindLoginsByID(store))).Methods(http.MethodGet)
	apiRouter.Handle("/logins/{id:[0-9]+}", contextMiddleware(UpdateLogin(store))).Methods(http.MethodPut)
	apiRouter.Handle("/logins/{id:[0-9]+}", contextMiddleware(DeleteLogin(store))).Methods(http.MethodDelete)

	return apiRouter
}

func contextMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctxWithID := context.WithValue(ctx, "id", 1)
		ctxWithAuthorized := context.WithValue(ctxWithID, "authorized", true)
		ctxWithSchema := context.WithValue(ctxWithAuthorized, "schema", "user-test")

		h.ServeHTTP(w, r.WithContext(ctxWithSchema))
	})
}

// func TestDeleteLogin(t *testing.T) {
// 	w := httptest.NewRecorder()

// 	// Create mock db
// 	mockDB, mock := dbSetup()

// 	// Initialize router
// 	r := routersSetup(mockDB)

// 	// Generate dummy login
// 	loginDTO := &model.LoginDTO{
// 		ID:       1,
// 		Title:    "Dummy Title",
// 		URL:      "http://dummy.com",
// 		Username: "dummyuser",
// 		Password: "dummypassword",
// 	}

// 	encryptedPassword := "GRr4f5bWKolEVw8EjXSryNPvVLEorL3VILyYhMUkZiize6FlBvP4C1I="

// 	// Add dummy login to dummy db table
// 	rows := sqlmock.
// 		NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "title", "url", "username", "password"}).
// 		AddRow(loginDTO.ID, time.Now(), time.Now(), nil, loginDTO.Title, loginDTO.URL, loginDTO.Username, encryptedPassword)

// 	// Define expected query
// 	const sqlSelectOne = `SELECT * FROM "user-test"."logins" WHERE "user-test"."logins"."deleted_at" IS NULL AND ((id = $1))`
// 	mock.ExpectQuery(regexp.QuoteMeta(sqlSelectOne)).
// 		WithArgs(loginDTO.ID).
// 		WillReturnRows(rows)

// 	// Define expected query
// 	const sqlDeleteOne = `DELETE FROM "user-test"."logins" WHERE "user-test"."logins"."deleted_at" IS NULL AND ((id = $1))`
// 	mock.ExpectBegin() // start transaction
// 	mock.ExpectQuery(regexp.QuoteMeta(sqlDeleteOne)).
// 		WithArgs(loginDTO.ID).
// 		WillReturnRows(rows)
// 	mock.ExpectCommit() // commit transaction

// 	// Make request
// 	r.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/logins/1", nil))

// 	// Check status code
// 	assert.Equal(t, http.StatusOK, w.Code, "Did not get expected HTTP status code, got")

// 	fmt.Println(w.Body.String())
// }

/* func TestCreateLogin(t *testing.T) {
	w := httptest.NewRecorder()

	// Create mock db
	mockDB, mock := dbSetup()
	mockDB.LogMode(true)

	// Initialize router
	r := routersSetup(mockDB)

	// Generate dummy login
	loginDTO := &model.LoginDTO{
		ID:       1,
		Title:    "Dummy Title",
		URL:      "http://dummy.com",
		Username: "dummyuser",
		Password: "dummypassword",
	}

	const sqlInsert = `INSERT INTO "user-test"."logins" ("created_at","updated_at","deleted_at","title","url","username","password") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "user-test"."logins"."id"`

	mock.ExpectBegin() // start transaction
	mock.ExpectQuery(regexp.QuoteMeta(sqlInsert)).
		WithArgs(AnyTime{}, AnyTime{}, nil, loginDTO.Title, loginDTO.URL, loginDTO.Username, loginDTO.Password).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(loginDTO.ID))
	mock.ExpectCommit() // commit transaction

	// Make request
	data, _ := json.Marshal(loginDTO)
	req, _ := http.NewRequest("POST", "/api/logins", bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
} */

type AnyTime struct{}

func (a AnyTime) Match(v driver.Value) bool {
	_, ok := v.(time.Time)
	return ok
}
