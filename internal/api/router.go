package api

import (
	"github.com/gorilla/mux"
	"github.com/urfave/negroni"

	"github.com/jinzhu/gorm"
	"github.com/pass-wall/passwall-server/internal/middleware"
	"github.com/pass-wall/passwall-server/internal/storage"
)

// Router ...
func Router() *negroni.Negroni {

	db := storage.GetDB()
	loginAPI := InitLoginAPI(db)

	router := mux.NewRouter()
	n := negroni.Classic()

	loginRouter := mux.NewRouter().PathPrefix("/api").Subrouter()
	loginRouter.HandleFunc("/logins", loginAPI.FindAll).Methods("GET")
	loginRouter.HandleFunc("/logins", loginAPI.Create).Methods("POST")
	loginRouter.HandleFunc("/logins/{id:[0-9]+}", loginAPI.FindByID).Methods("GET")
	loginRouter.HandleFunc("/logins/{id:[0-9]+}", loginAPI.Update).Methods("PUT")
	loginRouter.HandleFunc("/logins/{id:[0-9]+}", loginAPI.Delete).Methods("DELETE")
	loginRouter.HandleFunc("/logins/{action}", loginAPI.PostHandler).Methods("POST")
	loginRouter.HandleFunc("/logins/{action}", loginAPI.GetHandler).Methods("GET")

	authRouter := mux.NewRouter().PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/signin", Signin)
	authRouter.HandleFunc("/refresh", RefreshToken)
	authRouter.HandleFunc("/check", CheckToken)

	router.PathPrefix("/api").Handler(n.With(
		negroni.HandlerFunc(middleware.Auth),
		negroni.Wrap(loginRouter),
	))

	router.PathPrefix("/auth").Handler(n.With(
		negroni.Wrap(authRouter),
	))

	n.Use(negroni.HandlerFunc(middleware.CORS))
	n.UseHandler(router)

	return n
}

// InitLoginAPI ..
func InitLoginAPI(db *gorm.DB) LoginAPI {
	loginRepository := storage.NewLoginRepository(db)
	loginService := storage.NewLoginService(loginRepository)
	loginAPI := NewLoginAPI(loginService)
	loginAPI.Migrate()
	return loginAPI
}
