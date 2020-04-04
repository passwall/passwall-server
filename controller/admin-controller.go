package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Dashboard(c *gin.Context) {

	// Get user info from BasicAuth middleware
	// AuthUserKey is the cookie name for user credential in basic auth.
	user := c.MustGet(gin.AuthUserKey).(string)

	//Show some secret info
	c.JSON(http.StatusOK, gin.H{"Welcome: ": user})

}
