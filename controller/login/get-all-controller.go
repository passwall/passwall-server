package login

import (
	"log"

	"github.com/pass-wall/passwall-api/controller/helper"
	"github.com/pass-wall/passwall-api/model"
	"github.com/pass-wall/passwall-api/pkg/database"

	"github.com/gin-gonic/gin"
)

func GetLogins(c *gin.Context) {
	var logins []model.Login
	db = database.GetDB()

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

	/* if logins, err = repo.ListAll(); err != nil {
		log.Println(err)
		c.AbortWithStatus(404)
		return
	} */

	// Set Data result
	logins = helper.DecryptLoginPasswords(logins)

	c.JSON(200, logins)
}
