package controller

import (
	"fmt"
	"log"

	"github.com/yakuter/gpass/model"
	"github.com/yakuter/gpass/pkg/config"
	"github.com/yakuter/gpass/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
)

var db *gorm.DB
var err error

// Data is mainle generated for filtering and pagination
type Data struct {
	TotalData    int64
	FilteredData int64
	Data         []model.Login
}

func GetLogin(c *gin.Context) {
	db = database.GetDB()
	config := config.GetConfig()
	id := c.Params.ByName("id")
	var login model.Login

	if err := db.Where("id = ? ", id).First(&login).Error; err != nil {
		log.Println(err)
		c.AbortWithStatus(404)
		return
	}

	login.Password = decrypt(login.Password, config.Server.Salt)

	c.JSON(200, login)
}

func GetLogins(c *gin.Context) {
	db = database.GetDB()
	var logins []model.Login
	var data Data

	// Define and get sorting field
	sort := c.DefaultQuery("Sort", "ID")

	// Define and get sorting order field
	order := c.DefaultQuery("Order", "DESC")

	// Define and get offset for pagination
	offset := c.DefaultQuery("Offset", "0")

	// Define and get limit for pagination
	limit := c.DefaultQuery("Limit", "25")

	// Get search keyword for Search Scope
	search := c.DefaultQuery("Search", "")

	table := "logins"
	query := db.Select(table + ".*")
	query = query.Offset(Offset(offset))
	query = query.Limit(Limit(limit))
	query = query.Order(SortOrder(table, sort, order))
	query = query.Scopes(Search(search))

	if err := query.Find(&logins).Error; err != nil {
		log.Println(err)
		c.AbortWithStatus(404)
		return
	}
	// Count filtered table
	// We are resetting offset to 0 to return total number.
	// This is a fix for Gorm offset issue
	query = query.Offset(0)
	query.Table(table).Count(&data.FilteredData)

	// Count total table
	db.Table(table).Count(&data.TotalData)

	// Set Data result
	data.Data = DecryptLoginPasswords(logins)

	c.JSON(200, data)
}

func CreateLogin(c *gin.Context) {
	db = database.GetDB()
	config := config.GetConfig()
	var login model.Login

	c.BindJSON(&login)

	if login.Password == "" {
		login.Password = Password()
	}

	login.Password = encrypt(login.Password, config.Server.Salt)

	if err := db.Create(&login).Error; err != nil {
		fmt.Println(err)
		c.AbortWithStatus(404)
		return
	}

	login.Password = decrypt(login.Password, config.Server.Salt)

	c.JSON(200, login)
}

func UpdateLogin(c *gin.Context) {
	db = database.GetDB()
	config := config.GetConfig()
	var login model.Login
	id := c.Params.ByName("id")

	if err := db.Where("id = ?", id).First(&login).Error; err != nil {
		log.Println(err)
		c.AbortWithStatus(404)
		return
	}

	c.BindJSON(&login)

	if login.Password == "" {
		login.Password = Password()
	}
	login.Password = encrypt(login.Password, config.Server.Salt)

	db.Save(&login)
	c.JSON(200, login)
}

func DeleteLogin(c *gin.Context) {
	db = database.GetDB()
	id := c.Params.ByName("id")
	var login model.Login

	if err := db.Where("id = ? ", id).Delete(&login).Error; err != nil {
		log.Println(err)
		c.AbortWithStatus(404)
		return
	}

	c.JSON(200, gin.H{"id#" + id: "deleted"})
}
