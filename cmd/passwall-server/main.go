package main

import (
	"github.com/gorilla/mux"
	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/jinzhu/gorm"
	"github.com/pass-wall/passwall-server/internal/api"
	"github.com/pass-wall/passwall-server/internal/config"
	"github.com/pass-wall/passwall-server/internal/cron"
	"github.com/pass-wall/passwall-server/internal/middleware"
	"github.com/pass-wall/passwall-server/internal/store"
	"github.com/spf13/viper"
	"github.com/urfave/negroni"
)

func init() {
	config.Setup()
	store.Setup()
	cron.Setup()
}

func main() {

	db := store.GetDB()
	loginAPI := InitLoginAPI(db)

	router := mux.NewRouter()
	loginRouter := mux.NewRouter().PathPrefix("/api").Subrouter().StrictSlash(true)
	loginRouter.HandleFunc("/logins", loginAPI.FindAll).Methods("GET")
	loginRouter.HandleFunc("/logins", loginAPI.Create).Methods("POST")
	loginRouter.HandleFunc("/logins/{id:[0-9]+}", loginAPI.FindByID).Methods("GET")
	loginRouter.HandleFunc("/logins/{id:[0-9]+}", loginAPI.Update).Methods("PUT")
	loginRouter.HandleFunc("/logins/{id:[0-9]+}", loginAPI.Delete).Methods("DELETE")
	loginRouter.HandleFunc("/logins/{action}", loginAPI.PostHandler).Methods("POST")

	authRouter := mux.NewRouter().PathPrefix("/auth").Subrouter().StrictSlash(true)
	authRouter.HandleFunc("/signin", api.Signin)
	authRouter.HandleFunc("/refresh", api.RefreshToken)

	n := negroni.Classic()

	router.PathPrefix("/api").Handler(n.With(
		negroni.HandlerFunc(middleware.Auth),
		negroni.Wrap(loginRouter),
	))

	router.PathPrefix("/auth").Handler(n.With(
		negroni.Wrap(authRouter),
	))

	n.Use(negroni.HandlerFunc(middleware.CORS))
	n.UseHandler(router)

	n.Run(":" + viper.GetString("server.port"))

}

// InitLoginAPI ..
func InitLoginAPI(db *gorm.DB) api.LoginAPI {
	loginRepository := store.NewLoginRepository(db)
	loginService := store.NewLoginService(loginRepository)
	loginAPI := api.NewLoginAPI(loginService)
	loginAPI.Migrate()
	return loginAPI
}
