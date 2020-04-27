package main

import (
	"net/http"

	"github.com/gorilla/mux"
	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/jinzhu/gorm"
	"github.com/pass-wall/passwall-server/internal/api"
	"github.com/pass-wall/passwall-server/internal/config"
	"github.com/pass-wall/passwall-server/internal/cron"
	"github.com/pass-wall/passwall-server/internal/store"
	"github.com/spf13/viper"
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
	r.HandleFunc("/logins", loginAPI.FindAll).Methods("GET")
	r.HandleFunc("/logins", loginAPI.PostHandler).Methods("POST")
	r.HandleFunc("/logins/{id:[0-9]+}", loginAPI.FindByID).Methods("GET")
	r.HandleFunc("/logins/{id:[0-9]+}", loginAPI.Update).Methods("PUT")
	r.HandleFunc("/logins/{id:[0-9]+}", loginAPI.Delete).Methods("DELETE")
	r.HandleFunc("/logins/{action}", loginAPI.PostHandler).Methods("POST")

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

	http.ListenAndServe(":"+viper.GetString("server.port"), r)
}

// InitLoginAPI ..
func InitLoginAPI(db *gorm.DB) api.LoginAPI {
	loginRepository := store.NewLoginRepository(db)
	loginService := store.NewLoginService(loginRepository)
	loginAPI := api.NewLoginAPI(loginService)
	loginAPI.Migrate()
	return loginAPI
}
