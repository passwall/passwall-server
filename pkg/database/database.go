package database

import (
	"log"

	"github.com/spf13/viper"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var (
	// DB ...
	DB  *gorm.DB
	err error
)

// Database ...
type Database struct {
	*gorm.DB
}

// Setup opens a database and saves the reference to `Database` struct.
func Setup() {
	var db = DB

	driver := viper.GetString("database.driver")
	database := viper.GetString("database.dbname")
	username := viper.GetString("database.username")
	password := viper.GetString("database.password")
	host := viper.GetString("database.host")
	port := viper.GetString("database.port")

	if driver == "sqlite" {

		path := viper.GetString("database.path")
		if len(path) == 0 {
			log.Println("no database.path provided in config file for sqlite. using default:", path)
			path = "./store/passwall.db"
		}
		db, err = gorm.Open("sqlite3", path)
		if err != nil {
			log.Fatal("db err: ", err)
		}

	} else if driver == "postgres" {

		db, err = gorm.Open("postgres", "host="+host+" port="+port+" user="+username+" dbname="+database+"  sslmode=disable password="+password)
		if err != nil {
			log.Fatal("db err: ", err)
		}

	} else if driver == "mysql" {

		db, err = gorm.Open("mysql", username+":"+password+"@tcp("+host+":"+port+")/"+database+"?charset=utf8&parseTime=True&loc=Local")
		if err != nil {
			log.Fatal("db err: ", err)
		}

	}

	// Change this to true if you want to see SQL queries
	db.LogMode(false)

	DB = db
}

// GetDB helps you to get a connection
func GetDB() *gorm.DB {
	return DB
}
