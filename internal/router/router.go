package router

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/urfave/negroni"

	"github.com/pass-wall/passwall-server/internal/api"
	"github.com/pass-wall/passwall-server/internal/storage"
)

// Router ...
type Router struct {
	router *mux.Router
	store  storage.Store
}

// New ...
func New(s storage.Store) *Router {
	r := &Router{
		router: mux.NewRouter(),
		store:  s,
	}
	r.initRoutes()
	return r
}

// ServeHTTP ...
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}

func (r *Router) initRoutes() {
	// API Router Group
	apiRouter := mux.NewRouter().PathPrefix("/api").Subrouter()

	// Login endpoints
	apiRouter.HandleFunc("/logins", api.FindAllLogins(r.store)).Methods(http.MethodGet)
	apiRouter.HandleFunc("/logins", api.CreateLogin(r.store)).Methods(http.MethodPost)
	apiRouter.HandleFunc("/logins/{id:[0-9]+}", api.FindLoginsByID(r.store)).Methods(http.MethodGet)
	apiRouter.HandleFunc("/logins/{id:[0-9]+}", api.UpdateLogin(r.store)).Methods(http.MethodPut)
	apiRouter.HandleFunc("/logins/{id:[0-9]+}", api.DeleteLogin(r.store)).Methods(http.MethodDelete)

	// Bank Account endpoints
	apiRouter.HandleFunc("/bank-accounts", api.FindAllBankAccounts(r.store)).Methods(http.MethodGet)
	apiRouter.HandleFunc("/bank-accounts", api.CreateBankAccount(r.store)).Methods(http.MethodPost)
	apiRouter.HandleFunc("/bank-accounts/{id:[0-9]+}", api.FindBankAccountByID(r.store)).Methods(http.MethodGet)
	apiRouter.HandleFunc("/bank-accounts/{id:[0-9]+}", api.UpdateBankAccount(r.store)).Methods(http.MethodPut)
	apiRouter.HandleFunc("/bank-accounts/{id:[0-9]+}", api.DeleteBankAccount(r.store)).Methods(http.MethodDelete)

	// Credit Card endpoints
	apiRouter.HandleFunc("/credit-cards", api.FindAllCreditCards(r.store)).Methods(http.MethodGet)
	apiRouter.HandleFunc("/credit-cards", api.CreateCreditCard(r.store)).Methods(http.MethodPost)
	apiRouter.HandleFunc("/credit-cards/{id:[0-9]+}", api.FindCreditCardByID(r.store)).Methods(http.MethodGet)
	apiRouter.HandleFunc("/credit-cards/{id:[0-9]+}", api.UpdateCreditCard(r.store)).Methods(http.MethodPut)
	apiRouter.HandleFunc("/credit-cards/{id:[0-9]+}", api.DeleteCreditCard(r.store)).Methods(http.MethodDelete)

	// Note endpoints
	apiRouter.HandleFunc("/notes", api.FindAllNotes(r.store)).Methods(http.MethodGet)
	apiRouter.HandleFunc("/notes", api.CreateNote(r.store)).Methods(http.MethodPost)
	apiRouter.HandleFunc("/notes/{id:[0-9]+}", api.FindNoteByID(r.store)).Methods(http.MethodGet)
	apiRouter.HandleFunc("/notes/{id:[0-9]+}", api.UpdateNote(r.store)).Methods(http.MethodPut)
	apiRouter.HandleFunc("/notes/{id:[0-9]+}", api.DeleteNote(r.store)).Methods(http.MethodDelete)

	// Email endpoints
	apiRouter.HandleFunc("/emails", api.FindAllEmails(r.store)).Methods(http.MethodGet)
	apiRouter.HandleFunc("/emails", api.CreateEmail(r.store)).Methods(http.MethodPost)
	apiRouter.HandleFunc("/emails/{id:[0-9]+}", api.FindEmailByID(r.store)).Methods(http.MethodGet)
	apiRouter.HandleFunc("/emails/{id:[0-9]+}", api.UpdateEmail(r.store)).Methods(http.MethodPut)
	apiRouter.HandleFunc("/emails/{id:[0-9]+}", api.DeleteEmail(r.store)).Methods(http.MethodDelete)

	// System endpoint
	// TODO: Change these to system endpoints
	apiRouter.HandleFunc("/logins/check-password", api.FindSamePassword(r.store)).Methods(http.MethodPost)
	apiRouter.HandleFunc("/logins/generate-password", api.GeneratePassword).Methods(http.MethodPost)

	apiRouter.HandleFunc("/logins/backup", api.Backup(r.store)).Methods(http.MethodPost)
	apiRouter.HandleFunc("/logins/backup", api.ListBackup).Methods(http.MethodGet)
	apiRouter.HandleFunc("/logins/restore", api.Restore(r.store)).Methods(http.MethodPost)

	apiRouter.HandleFunc("/logins/import", api.Import(r.store)).Methods(http.MethodPost)
	apiRouter.HandleFunc("/logins/export", api.Export(r.store)).Methods(http.MethodPost)

	// Auth endpoints
	authRouter := mux.NewRouter().PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/signin", api.Signin(r.store))
	authRouter.HandleFunc("/refresh", api.RefreshToken(r.store))
	authRouter.HandleFunc("/check", api.CheckToken)

	n := negroni.Classic()
	n.Use(negroni.HandlerFunc(CORS))
	n.Use(negroni.HandlerFunc(Secure))

	r.router.PathPrefix("/api").Handler(n.With(
		Auth(r.store),
		negroni.Wrap(apiRouter),
	))

	r.router.PathPrefix("/auth").Handler(n.With(

		LimitHandler(),
		negroni.Wrap(authRouter),
	))

	r.router.HandleFunc("/health", api.HealthCheck(r.store)).Methods(http.MethodGet)
}
