package middleware

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
/* func AuthMiddleware() *jwt.GinJWTMiddleware {
	usernameCfg := viper.GetString("server.username")
	passwordCfg := viper.GetString("server.password")
	secret := viper.GetString("server.secret")
	timeout := viper.GetInt("server.timeout")
	if timeout < 0 {
		timeout = 1
	}

	authMiddleware, err := jwt.New(&jwt.GinJWTMiddleware{
		Realm:       "PassWall Area",
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
		IdentityHandler: func(w http.ResponseWriter, r *http.Request) interface{} {
			claims := jwt.ExtractClaims(c)
			return &User{
				Username: claims[identityKey].(string),
			}
		},
		Authenticator: func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
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
		Authorizator: func(data interface{}, w http.ResponseWriter, r *http.Request) bool {
			if v, ok := data.(*User); ok && v.Username == usernameCfg {
				return true
			}

			return false
		},
		Unauthorized: func(w http.ResponseWriter, r *http.Request, code int, message string) {
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

// TokenCheck ...
func TokenCheck(w http.ResponseWriter, r *http.Request) {
	// result := login.Result{"Success", "Token is valid"}
	// c.JSON(http.StatusOK, result)
	c.JSON(http.StatusOK, gin.H{"Status": "Success", "Message": "Token is valid"})
}
*/
