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
	apiRouter.HandleFunc("/logins", api.FindAllLogins(r.store)).Methods("GET")
	apiRouter.HandleFunc("/logins", api.CreateLogin(r.store)).Methods("POST")
	apiRouter.HandleFunc("/logins/{id:[0-9]+}", api.FindLoginsByID(r.store)).Methods("GET")
	apiRouter.HandleFunc("/logins/{id:[0-9]+}", api.UpdateLogin(r.store)).Methods("PUT")
	apiRouter.HandleFunc("/logins/{id:[0-9]+}", api.DeleteLogin(r.store)).Methods("DELETE")

	// Bank Account endpoints
	apiRouter.HandleFunc("/bank-accounts", api.FindAllBankAccounts(r.store)).Methods("GET")
	apiRouter.HandleFunc("/bank-accounts", api.CreateBankAccount(r.store)).Methods("POST")
	apiRouter.HandleFunc("/bank-accounts/{id:[0-9]+}", api.FindBankAccountByID(r.store)).Methods("GET")
	apiRouter.HandleFunc("/bank-accounts/{id:[0-9]+}", api.UpdateBankAccount(r.store)).Methods("PUT")
	apiRouter.HandleFunc("/bank-accounts/{id:[0-9]+}", api.DeleteBankAccount(r.store)).Methods("DELETE")

	// Credit Card endpoints
	apiRouter.HandleFunc("/credit-cards", api.FindAllCreditCards(r.store)).Methods("GET")
	apiRouter.HandleFunc("/credit-cards", api.CreateCreditCard(r.store)).Methods("POST")
	apiRouter.HandleFunc("/credit-cards/{id:[0-9]+}", api.FindCreditCardByID(r.store)).Methods("GET")
	apiRouter.HandleFunc("/credit-cards/{id:[0-9]+}", api.UpdateCreditCard(r.store)).Methods("PUT")
	apiRouter.HandleFunc("/credit-cards/{id:[0-9]+}", api.DeleteCreditCard(r.store)).Methods("DELETE")

	// Note endpoints
	apiRouter.HandleFunc("/notes", api.FindAllNotes(r.store)).Methods("GET")
	apiRouter.HandleFunc("/notes", api.CreateNote(r.store)).Methods("POST")
	apiRouter.HandleFunc("/notes/{id:[0-9]+}", api.FindNoteByID(r.store)).Methods("GET")
	apiRouter.HandleFunc("/notes/{id:[0-9]+}", api.UpdateNote(r.store)).Methods("PUT")
	apiRouter.HandleFunc("/notes/{id:[0-9]+}", api.DeleteNote(r.store)).Methods("DELETE")

	// System endpoint
	// TODO: Change these to system endpoints
	apiRouter.HandleFunc("/logins/check-password", api.FindSamePassword(r.store)).Methods("POST")
	apiRouter.HandleFunc("/logins/generate-password", api.GeneratePassword).Methods("POST")

	apiRouter.HandleFunc("/logins/backup", api.Backup(r.store)).Methods("POST")
	apiRouter.HandleFunc("/logins/backup", api.ListBackup).Methods("GET")
	apiRouter.HandleFunc("/logins/restore", api.Restore(r.store)).Methods("POST")

	apiRouter.HandleFunc("/logins/import", api.Import(r.store)).Methods("POST")
	apiRouter.HandleFunc("/logins/export", api.Export(r.store)).Methods("POST")

	// Auth endpoints
	authRouter := mux.NewRouter().PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/signin", api.Signin)
	authRouter.HandleFunc("/refresh", api.RefreshToken)
	authRouter.HandleFunc("/check", api.CheckToken)

	n := negroni.Classic()
	n.Use(negroni.HandlerFunc(CORS))

	r.router.PathPrefix("/api").Handler(n.With(
		negroni.HandlerFunc(Auth),
		negroni.Wrap(apiRouter),
	))

	r.router.PathPrefix("/auth").Handler(n.With(
		negroni.HandlerFunc(LimitHandler()),
		negroni.Wrap(authRouter),
	))
}
