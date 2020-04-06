package login

import (
	"fmt"

	"github.com/yakuter/gpass/controller/helper"
	"github.com/yakuter/gpass/model"
	"github.com/yakuter/gpass/pkg/config"
	"github.com/yakuter/gpass/pkg/database"

	"github.com/gin-gonic/gin"
)

func CreateLogin(c *gin.Context) {
	db = database.GetDB()
	config := config.GetConfig()
	var login model.Login

	c.BindJSON(&login)

	if login.Password == "" {
		login.Password = helper.Password()
	}

	login.Password = helper.Encrypt(login.Password, config.Server.Passphrase)

	if err := db.Create(&login).Error; err != nil {
		fmt.Println(err)
		c.AbortWithStatus(404)
		return
	}

	login.Password = helper.Decrypt(login.Password, config.Server.Passphrase)

	c.JSON(200, login)
}
