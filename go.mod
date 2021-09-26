module github.com/passwall/passwall-server

go 1.14

require (
	github.com/Luzifer/go-openssl/v4 v4.1.0
	github.com/didip/tollbooth v4.0.2+incompatible
	github.com/fatih/color v1.13.0
	github.com/go-playground/validator/v10 v10.9.0
	github.com/go-test/deep v1.0.7
	github.com/golang-jwt/jwt/v4 v4.1.0
	github.com/gorilla/mux v1.8.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/satori/go.uuid v1.2.0
	github.com/sendgrid/rest v2.6.5+incompatible // indirect
	github.com/sendgrid/sendgrid-go v3.10.1+incompatible
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/viper v1.9.0
	github.com/stretchr/testify v1.7.0
	github.com/urfave/negroni v1.0.0
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	gopkg.in/yaml.v2 v2.4.0
	gorm.io/driver/postgres v1.1.1
	gorm.io/gorm v1.21.15
)

replace (
	gopkg.in/russross/blackfriday.v2 v2.0.1 => github.com/russross/blackfriday/v2 v2.0.1
	gopkg.in/russross/blackfriday.v2 v2.1.0 => github.com/russross/blackfriday/v2 v2.1.0
)
