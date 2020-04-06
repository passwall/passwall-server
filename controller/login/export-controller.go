package login

import (
	"encoding/base64"
	"log"

	"github.com/yakuter/gpass/controller/helper"
	"github.com/yakuter/gpass/model"
	"github.com/yakuter/gpass/pkg/config"
	"github.com/yakuter/gpass/pkg/database"

	"github.com/gin-gonic/gin"
)

func Export(c *gin.Context) {
	db = database.GetDB()
	config := config.GetConfig()
	id := c.Params.ByName("id")
	var login model.Login

	if err := db.Where("id = ? ", id).First(&login).Error; err != nil {
		log.Println(err)
		c.AbortWithStatus(404)
		return
	}

	passByte, _ := base64.StdEncoding.DecodeString(login.Password)
	passB64 := helper.Decrypt(string(passByte[:]), config.Server.Passphrase)
	login.Password = passB64

	c.JSON(200, login)
}
