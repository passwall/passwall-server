module github.com/passwall/passwall-server

go 1.14

require (
	github.com/DATA-DOG/go-sqlmock v1.4.1
	github.com/Luzifer/go-openssl/v4 v4.1.0
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/didip/tollbooth v4.0.2+incompatible
	github.com/go-playground/validator/v10 v10.2.0
	github.com/go-test/deep v1.0.6
	github.com/google/uuid v1.2.0 // indirect
	github.com/gorilla/mux v1.7.4
	github.com/jinzhu/gorm v1.9.15
	github.com/mattn/go-sqlite3 v2.0.1+incompatible // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/satori/go.uuid v1.2.0
	github.com/sendgrid/rest v2.6.2+incompatible // indirect
	github.com/sendgrid/sendgrid-go v3.7.0+incompatible
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.6.1
	github.com/urfave/negroni v1.0.0
	golang.org/x/crypto v0.0.0-20210314154223-e6e6c4f2bb5b
	golang.org/x/sys v0.0.0-20210315160823-c6e025ad8005 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v2 v2.3.0
)

replace (
	gopkg.in/russross/blackfriday.v2 v2.0.1 => github.com/russross/blackfriday/v2 v2.0.1
	gopkg.in/russross/blackfriday.v2 v2.1.0 => github.com/russross/blackfriday/v2 v2.1.0
)
