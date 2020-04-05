package database

import (
	"fmt"

	"github.com/yakuter/gpass/model"
	"github.com/yakuter/gpass/pkg/config"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var (
	DB    *gorm.DB
	err   error
	DBErr error
)

type Database struct {
	*gorm.DB
}

// Setup opens a database and saves the reference to `Database` struct.
func Setup() {
	var db = DB

	Config := config.GetConfig()

	driver := Config.Database.Driver
	database := Config.Database.Dbname
	username := Config.Database.Username
	password := Config.Database.Password
	host := Config.Database.Host
	port := Config.Database.Port

	if driver == "sqlite" {
		db, err = gorm.Open("sqlite3", "./"+database+".db")

		if err != nil {
			DBErr = err
			fmt.Println("db err: ", err)
		}

	} else if driver == "postgres" {

		db, err = gorm.Open("postgres", "host="+host+" port="+port+" user="+username+" dbname="+database+"  sslmode=disable password="+password)
		if err != nil {
			DBErr = err
			fmt.Println("db err: ", err)
		}

	} else if driver == "mysql" {

		db, err = gorm.Open("mysql", username+":"+password+"@tcp("+host+":"+port+")/"+database+"?charset=utf8&parseTime=True&loc=Local")
		if err != nil {
			DBErr = err
			fmt.Println("db err: ", err)
		}

	}

	// Change this to true if you want to see SQL queries
	db.LogMode(false)

	// Auto migrate project models
	// db.DropTableIfExists(&model.Login{})
	db.AutoMigrate(&model.Login{})
	DB = db
}

// GetDB helps you to get a connection
func GetDB() *gorm.DB {
	return DB
}

// GetDBErr helps you to get a connection
func GetDBErr() error {
	return DBErr
}
