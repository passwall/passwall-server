package storage

import (
	"fmt"
	"log"

	"github.com/spf13/viper"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var (
	// DB ...
	DB            *gorm.DB
	err           error
	InvalidDriver = "Invalid database driver"
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

	switch driver := driver; driver {

	case "sqlite":
		// Default value is set on configuration.go
		path := viper.GetString("database.path")
		db, err = gorm.Open("sqlite3", path)
		FatalDBErr(err)

	case "postgres":

		db, err = gorm.Open("postgres", "host="+host+" port="+port+" user="+username+" dbname="+database+"  sslmode=disable password="+password)
		FatalDBErr(err)

	case "mysql":
		db, err = gorm.Open("mysql", username+":"+password+"@tcp("+host+":"+port+")/"+database+"?charset=utf8&parseTime=True&loc=Local")
		FatalDBErr(err)

	default:
		// if db driver did not specified or not supported
		fmt.Printf("Invalid database driver %s", driver)
	}

	// Change this to true if you want to see SQL queries
	db.LogMode(false)

	DB = db
}

// GetDB helps you to get a connection
func GetDB() *gorm.DB {
	return DB
}

func FatalDBErr(err error) {
	if err != nil {
		log.Fatal("db err: ", err)
	}
}
