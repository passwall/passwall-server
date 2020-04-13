package login

import (
	"encoding/base64"
	"fmt"

	"github.com/pass-wall/passwall-api/controller/helper"
	"github.com/pass-wall/passwall-api/model"
	"github.com/pass-wall/passwall-api/pkg/config"
	"github.com/pass-wall/passwall-api/pkg/database"

	"github.com/gin-gonic/gin"
)

func CreateLogin(c *gin.Context) {
	db = database.GetDB()
	config := config.GetConfig()
	var login *model.Login
	var rawPass string
	c.BindJSON(&login)

	if login.Password == "" {
		login.Password = helper.Password()
	}

	rawPass = login.Password
	login.Password = base64.StdEncoding.EncodeToString(helper.Encrypt(login.Password, config.Server.Passphrase))

	if err := db.Create(login).Error; err != nil {
		fmt.Println(err)
		c.AbortWithStatus(404)
		return
	}

	login.Password = rawPass

	c.JSON(200, login)
}
