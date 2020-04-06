package login

import (
	"log"

	"github.com/yakuter/gpass/controller/helper"
	"github.com/yakuter/gpass/model"
	"github.com/yakuter/gpass/pkg/database"

	"github.com/gin-gonic/gin"
)

// Data is mainle generated for filtering and pagination
type Data struct {
	TotalData    int64
	FilteredData int64
	Data         []model.Login
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
	query := db.Table(table)
	query = query.Select(table + ".*")
	query = query.Offset(helper.Offset(offset))
	query = query.Limit(helper.Limit(limit))
	query = query.Order(helper.SortOrder(table, sort, order))
	query = query.Scopes(helper.Search(search))

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
	data.Data = helper.DecryptLoginPasswords(logins)

	c.JSON(200, data)
}
