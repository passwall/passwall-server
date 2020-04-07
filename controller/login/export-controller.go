package login

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/yakuter/gpass/model"
	"github.com/yakuter/gpass/pkg/database"

	"github.com/gin-gonic/gin"
)

func Export(c *gin.Context) {
	db := database.GetDB()

	var logins []model.Login

	db.Find(&logins)

	file, err := os.Create("store/export_temp.csv")
	if err != nil {
		log.Println(err)
	}

	file.WriteString("URL,Username,Password\n")

	for _, login := range logins {
		_, err := file.WriteString(fmt.Sprintf("%s,%s,%s\n", login.URL, login.Username, login.Password))

		if err != nil {
			log.Println(err)
		}

	}

	c.File("store/export_temp.csv")
	c.Status(http.StatusOK)

	file.Close()
}
