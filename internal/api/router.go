package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/urfave/negroni"

	"github.com/pass-wall/passwall-server/internal/middleware"
	"github.com/pass-wall/passwall-server/internal/storage"
)

type Router struct {
	router *mux.Router
	store  storage.Store
}

// Router ...
func New(s storage.Store) *Router {
	r := &Router{
		router: mux.NewRouter(),
		store:  s,
	}
	r.initRoutes()
	return r
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}

func (r *Router) initRoutes() {
	// API Router Group
	apiRouter := mux.NewRouter().PathPrefix("/api").Subrouter()

	// Login endpoints
	apiRouter.HandleFunc("/logins", FindAllLogins(r.store)).Methods("GET")
	apiRouter.HandleFunc("/logins", CreateLogin(r.store)).Methods("POST")
	apiRouter.HandleFunc("/logins/{id:[0-9]+}", FindLoginsByID(r.store)).Methods("GET")
	apiRouter.HandleFunc("/logins/{id:[0-9]+}", UpdateLogin(r.store)).Methods("PUT")
	apiRouter.HandleFunc("/logins/{id:[0-9]+}", DeleteLogin(r.store)).Methods("DELETE")

	apiRouter.HandleFunc("/logins/check-password", FindSamePassword(r.store)).Methods("POST")
	apiRouter.HandleFunc("/logins/generate-password", GeneratePassword).Methods("POST")

	apiRouter.HandleFunc("/logins/backup", Backup(r.store)).Methods("POST")
	apiRouter.HandleFunc("/logins/backup", ListBackup).Methods("GET")

	apiRouter.HandleFunc("/logins/import", Import(r.store)).Methods("POST")
	apiRouter.HandleFunc("/logins/export", Export(r.store)).Methods("POST")
	apiRouter.HandleFunc("/logins/export", Restore(r.store)).Methods("POST")

	// Bank Account endpoints
	apiRouter.HandleFunc("/bank-accounts", FindAllBankAccounts(r.store)).Methods("GET")
	apiRouter.HandleFunc("/bank-accounts", CreateBankAccount(r.store)).Methods("POST")
	apiRouter.HandleFunc("/bank-accounts/{id:[0-9]+}", FindBankAccountByID(r.store)).Methods("GET")
	apiRouter.HandleFunc("/bank-accounts/{id:[0-9]+}", UpdateBankAccount(r.store)).Methods("PUT")
	apiRouter.HandleFunc("/bank-accounts/{id:[0-9]+}", DeleteBankAccount(r.store)).Methods("DELETE")

	apiRouter.HandleFunc("/bank-accounts/backup", ListBackup).Methods("GET")

	// Credit Card endpoints
	apiRouter.HandleFunc("/credit-cards", FindAllCreditCards(r.store)).Methods("GET")
	apiRouter.HandleFunc("/credit-cards", CreateCreditCard(r.store)).Methods("POST")
	apiRouter.HandleFunc("/credit-cards/{id:[0-9]+}", FindCreditCardByID(r.store)).Methods("GET")
	apiRouter.HandleFunc("/credit-cards/{id:[0-9]+}", UpdateCreditCard(r.store)).Methods("PUT")
	apiRouter.HandleFunc("/credit-cards/{id:[0-9]+}", DeleteCreditCard(r.store)).Methods("DELETE")

	apiRouter.HandleFunc("/credit-cards/backup", ListBackup).Methods("GET")

	// Note endpoints
	apiRouter.HandleFunc("/notes", noteAPI.FindAll).Methods("GET")
	apiRouter.HandleFunc("/notes", noteAPI.Create).Methods("POST")
	apiRouter.HandleFunc("/notes/{id:[0-9]+}", noteAPI.FindByID).Methods("GET")
	apiRouter.HandleFunc("/notes/{id:[0-9]+}", noteAPI.Update).Methods("PUT")
	apiRouter.HandleFunc("/notes/{id:[0-9]+}", noteAPI.Delete).Methods("DELETE")
	apiRouter.HandleFunc("/notes/{action}", noteAPI.GetHandler).Methods("GET")

	authRouter := mux.NewRouter().PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/signin", Signin)
	authRouter.HandleFunc("/refresh", RefreshToken)
	authRouter.HandleFunc("/check", CheckToken)

	n := negroni.Classic()
	n.Use(negroni.HandlerFunc(middleware.CORS))

	r.router.PathPrefix("/api").Handler(n.With(
		negroni.HandlerFunc(middleware.Auth),
		negroni.Wrap(apiRouter),
	))

	r.router.PathPrefix("/auth").Handler(n.With(
		negroni.HandlerFunc(middleware.LimitHandler()),
		negroni.Wrap(authRouter),
	))
}

// InitNoteAPI ..
func InitNoteAPI(db *gorm.DB) NoteAPI {
	noteRepository := storage.NewNoteRepository(db)
	noteService := app.NewNoteService(noteRepository)
	noteAPI := NewNoteAPI(noteService)
	noteAPI.Migrate()
	return noteAPI
}
