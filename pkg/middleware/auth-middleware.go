package middleware

import (
	"log"
	"net/http"
	"time"

	"github.com/yakuter/gpass/model"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/yakuter/gpass/pkg/config"
)

var err error

type login struct {
	Username string `form:"Username" json:"Username" binding:"required"`
	Password string `form:"Password" json:"Password" binding:"required"`
}

// User demo
type User struct {
	Username string
	Password string
}

var identityKey = "username"

// AuthMiddleware is JWT authorization middleware
func AuthMiddleware() *jwt.GinJWTMiddleware {

	config := config.GetConfig()
	usernameCfg := config.Server.Username
	passwordCfg := config.Server.Password
	secret := config.Server.Secret
	timeout := config.Server.Timeout
	if timeout < 0 {
		timeout = 1
	}

	authMiddleware, err := jwt.New(&jwt.GinJWTMiddleware{
		Realm:       "Gpass Area",
		Key:         []byte(secret),
		Timeout:     time.Duration(timeout) * time.Hour,
		MaxRefresh:  time.Hour,
		IdentityKey: identityKey,
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if v, ok := data.(*User); ok {
				return jwt.MapClaims{
					identityKey: v.Username,
				}
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			return &User{
				Username: claims[identityKey].(string),
			}
		},
		Authenticator: func(c *gin.Context) (interface{}, error) {
			var loginVals login
			if err := c.ShouldBind(&loginVals); err != nil {
				return "", jwt.ErrMissingLoginValues
			}
			userID := loginVals.Username
			password := loginVals.Password

			if userID == usernameCfg && password == passwordCfg {
				return &User{
					Username: usernameCfg,
					Password: passwordCfg,
				}, nil
			}

			return nil, jwt.ErrFailedAuthentication
		},
		Authorizator: func(data interface{}, c *gin.Context) bool {
			if v, ok := data.(*User); ok && v.Username == usernameCfg {
				return true
			}

			return false
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			c.JSON(code, gin.H{
				"code":    code,
				"message": message,
			})
		},
		// TokenLookup is a string in the form of "<source>:<name>" that is used
		// to extract token from the request.
		// Optional. Default value "header:Authorization".
		// Possible values:
		// - "header:<name>"
		// - "query:<name>"
		// - "cookie:<name>"
		// - "param:<name>"
		TokenLookup: "header: Authorization, query: token, cookie: jwt",
		// TokenLookup: "query:token",
		// TokenLookup: "cookie:token",

		// TokenHeadName is a string in the header. Default value is "Bearer"
		TokenHeadName: "Bearer",

		// TimeFunc provides the current time. You can override it to use another time value. This is useful for testing or if your server uses a different time zone than your tokens.
		TimeFunc: time.Now,
	})

	if err != nil {
		log.Fatal("JWT Error:" + err.Error())
	}

	return authMiddleware
}

func TokenCheck(c *gin.Context) {
	result := model.Result{"Success", "Token is valid"}
	c.JSON(http.StatusOK, result)
}
