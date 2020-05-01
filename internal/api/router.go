package api

import (
	"github.com/gorilla/mux"
	"github.com/urfave/negroni"

	"github.com/jinzhu/gorm"
	"github.com/pass-wall/passwall-server/internal/middleware"
	"github.com/pass-wall/passwall-server/internal/storage"
)

// Router ...
func Router() *mux.Router {

	db := storage.GetDB()
	loginAPI := InitLoginAPI(db)
	bankAccountAPI := InitBankAccountAPI(db)

	router := mux.NewRouter()
	n := negroni.Classic()
	n.Use(negroni.HandlerFunc(middleware.CORS))

	// API Router Group
	apiRouter := mux.NewRouter().PathPrefix("/api").Subrouter()

	// Login endpoints
	apiRouter.HandleFunc("/logins", loginAPI.FindAll).Methods("GET")
	apiRouter.HandleFunc("/logins", loginAPI.Create).Methods("POST")
	apiRouter.HandleFunc("/logins/{id:[0-9]+}", loginAPI.FindByID).Methods("GET")
	apiRouter.HandleFunc("/logins/{id:[0-9]+}", loginAPI.Update).Methods("PUT")
	apiRouter.HandleFunc("/logins/{id:[0-9]+}", loginAPI.Delete).Methods("DELETE")
	apiRouter.HandleFunc("/logins/{action}", loginAPI.PostHandler).Methods("POST")
	apiRouter.HandleFunc("/logins/{action}", loginAPI.GetHandler).Methods("GET")

	// Bank Account endpoints
	apiRouter.HandleFunc("/bank-accounts", bankAccountAPI.FindAll).Methods("GET")
	apiRouter.HandleFunc("/bank-accounts", bankAccountAPI.Create).Methods("POST")
	apiRouter.HandleFunc("/bank-accounts/{id:[0-9]+}", bankAccountAPI.FindByID).Methods("GET")
	apiRouter.HandleFunc("/bank-accounts/{id:[0-9]+}", bankAccountAPI.Update).Methods("PUT")
	apiRouter.HandleFunc("/bank-accounts/{id:[0-9]+}", bankAccountAPI.Delete).Methods("DELETE")
	apiRouter.HandleFunc("/bank-accounts/{action}", bankAccountAPI.GetHandler).Methods("GET")

	authRouter := mux.NewRouter().PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/signin", Signin)
	authRouter.HandleFunc("/refresh", RefreshToken)
	authRouter.HandleFunc("/check", CheckToken)

	router.PathPrefix("/api").Handler(n.With(
		negroni.HandlerFunc(middleware.Auth),
		negroni.Wrap(apiRouter),
	))

	router.PathPrefix("/auth").Handler(n.With(
		negroni.Wrap(authRouter),
	))

	return router
}

// InitLoginAPI ..
func InitLoginAPI(db *gorm.DB) LoginAPI {
	loginRepository := storage.NewLoginRepository(db)
	loginService := storage.NewLoginService(loginRepository)
	loginAPI := NewLoginAPI(loginService)
	loginAPI.Migrate()
	return loginAPI
}

// InitBankAccountAPI ..
func InitBankAccountAPI(db *gorm.DB) BankAccountAPI {
	bankAccountRepository := storage.NewBankAccountRepository(db)
	bankAccountService := storage.NewBankAccountService(bankAccountRepository)
	bankAccountAPI := NewBankAccountAPI(bankAccountService)
	bankAccountAPI.Migrate()
	return bankAccountAPI
}
