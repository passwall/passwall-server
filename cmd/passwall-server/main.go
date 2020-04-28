package main

import (
	"net/http"

	jwtmiddleware "github.com/auth0/go-jwt-middleware"
	"github.com/dgrijalva/jwt-go"
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

	r := mux.NewRouter()
	ar := mux.NewRouter()

	jwtMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return []byte("secret"), nil
		},
		SigningMethod: jwt.SigningMethodHS256,
	})

	ar.HandleFunc("/api/with-auth", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("auth required\n"))
	}).Methods("GET")

	r.HandleFunc("/logins", loginAPI.FindAll).Methods("GET")
	r.HandleFunc("/logins", loginAPI.PostHandler).Methods("POST")
	r.HandleFunc("/logins/{id:[0-9]+}", loginAPI.FindByID).Methods("GET")
	r.HandleFunc("/logins/{id:[0-9]+}", loginAPI.Update).Methods("PUT")
	r.HandleFunc("/logins/{id:[0-9]+}", loginAPI.Delete).Methods("DELETE")
	r.HandleFunc("/logins/{action}", loginAPI.PostHandler).Methods("POST")

	an := negroni.New(negroni.HandlerFunc(jwtMiddleware.HandlerWithNext), negroni.Wrap(ar))
	r.PathPrefix("/api").Handler(an)

	// negroni.Classic includes these default middlewares:
	// negroni.Recovery - Panic Recovery Middleware.
	// negroni.Logger - Request/Response Logger Middleware.
	// negroni.Static - Static File serving under the "public" directory.
	n := negroni.Classic()
	n.Use(negroni.HandlerFunc(jwtMiddleware.HandlerWithNext))
	n.Use(negroni.Wrap(r))
	n.Use(negroni.HandlerFunc(middleware.CORS))
	n.UseHandler(r)

	// logins.GET("/", loginAPI.FindAll)
	// logins.GET("/:id", loginAPI.FindByID)
	// logins.POST("/", loginAPI.Create)
	// logins.POST("/:action", func(w http.ResponseWriter, r *http.Request) {
	// 	path := c.Param("action")
	// 	if path == "check-password" {
	// 		loginAPI.FindSamePassword(c)
	// 	} else {
	// 		postHandler(c)
	// 	}
	// })

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
