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

	if err := db.Find(&logins).Error; err != nil {
		log.Println(err)
		c.AbortWithStatus(404)
		return
	}

	// Set Data result
	logins = helper.DecryptLoginPasswords(logins)

	c.JSON(200, logins)
}
