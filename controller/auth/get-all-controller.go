package login

import (
	"log"

	"github.com/yakuter/gpass/controller/helper"
	"github.com/yakuter/gpass/model"
	"github.com/yakuter/gpass/pkg/database"

	"github.com/gin-gonic/gin"
)

func GetLogins(c *gin.Context) {
	db = database.GetDB()
	var logins []model.Login

	// Get search keyword for Search Scope
	// This not for frontend, this is for browser extensions
	search := c.DefaultQuery("Search", "")

	table := "logins"
	query := db.Select(table + ".*")
	query = query.Order(helper.SortOrder(table, "id", "DESC"))
	query = query.Scopes(helper.Search(search))

	if err := query.Find(&logins).Error; err != nil {
		log.Println(err)
		c.AbortWithStatus(404)
		return
	}

	// Set Data result
	logins = helper.DecryptLoginPasswords(logins)

	c.JSON(200, logins)
}
