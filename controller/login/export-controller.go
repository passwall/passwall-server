package login

import (
	"github.com/gin-gonic/gin"
)

// Export exports all logins as CSV file
func Export(c *gin.Context) {
	/* db := database.GetDB()

	var logins []model.Login
	filepath := "/tmp/passwall_api_export.csv"

	db.Find(&logins)
	helper.DecryptLoginPasswords(&logins)

	file, err := os.Create(filepath)
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

	c.File(filepath)
	c.Status(http.StatusOK)

	file.Close() */
}
