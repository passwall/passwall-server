package login

import (
	"log"

	"github.com/yakuter/gpass/controller/helper"
	"github.com/yakuter/gpass/model"
	"github.com/yakuter/gpass/pkg/config"
	"github.com/yakuter/gpass/pkg/database"

	"github.com/gin-gonic/gin"
)

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
		login.Password = helper.Password()
	}
	login.Password = helper.Encrypt(login.Password, config.Server.Passphrase)

	db.Save(&login)
	c.JSON(200, login)
}
