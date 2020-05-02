package app

import (
	"bytes"
	"encoding/csv"
	"net/http"

	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
)

// Export exports all logins as CSV file
func Export(w http.ResponseWriter, r *http.Request) {
	db := storage.GetDB()

	var logins []model.Login
	db.Find(&logins)
	logins = DecryptLoginPasswords(logins)

	content := [][]string{}
	content = append(content, []string{"URL", "Username", "Password"})
	for i := range logins {
		content = append(content, []string{logins[i].URL, logins[i].Username, logins[i].Password})
	}

	b := &bytes.Buffer{} // creates IO Writer
	csvWriter := csv.NewWriter(b)
	strWrite := content
	csvWriter.WriteAll(strWrite)
	csvWriter.Flush()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename=PassWall.csv")
	w.Write(b.Bytes())
}
