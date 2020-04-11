package login

import (
	"log"

	"github.com/yakuter/gpass/model"
	"github.com/yakuter/gpass/pkg/database"

	"github.com/gin-gonic/gin"
)

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
