package login

import (
	"encoding/base64"
	"log"
	"net/http"
	"strconv"

	"github.com/yakuter/gpass/controller/helper"
	"github.com/yakuter/gpass/model"
	"github.com/yakuter/gpass/pkg/config"
	"github.com/yakuter/gpass/pkg/database"

	"github.com/gin-gonic/gin"
)

func GetLogin(c *gin.Context) {
	db = database.GetDB()
	config := config.GetConfig()
	id := c.Params.ByName("id")
	var login model.Login

	idInt, err := strconv.Atoi(id)
	if err != nil {
		result := model.Result{"Error", err.Error()}
		c.JSON(http.StatusBadRequest, result)
		return
	}

	if err := db.Where("id = ? ", idInt).First(&login).Error; err != nil {
		log.Println(err)
		result := model.Result{"Error", err.Error()}
		c.JSON(http.StatusNotFound, result)
		return
	}

	passByte, _ := base64.StdEncoding.DecodeString(login.Password)
	passB64 := helper.Decrypt(string(passByte[:]), config.Server.Passphrase)
	login.Password = passB64

	c.JSON(200, login)
}
