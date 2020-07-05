package api

import (
	"net/http"

	"github.com/passwall/passwall-server/internal/storage"
)

var (
	Port          = "3625"
	ServerAddress = "0.0.0.0" + ":" + Port
)

// HealthProp ...
type HealthProp struct {
	StatusCode int   `json:"status_code"`
	Err        error `json:"error"`
}

// Services ...
type Services struct {
	API      *HealthProp `json:"api"`
	Database *HealthProp `json:"database"`
}

// HealthCheck ...
func HealthCheck(s storage.Store) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		var checkResult Services
		var APIStatus *HealthProp
		var DBStatus *HealthProp

		if err := checkEndPoint(ServerAddress); err != nil {
			APIStatus = getStatus(http.StatusInternalServerError, err)
		}

		APIStatus = getStatus(http.StatusOK, nil)

		if err := s.Ping(); err != nil {
			DBStatus = getStatus(http.StatusInternalServerError, err)
		}

		DBStatus = getStatus(http.StatusOK, nil)

		checkResult = Services{
			API:      APIStatus,
			Database: DBStatus,
		}

		RespondWithJSON(w, checkResult.Database.StatusCode, checkResult)
	}

}

func checkEndPoint(url string) error {
	_, err := http.Get(url)
	if err != nil {
		return err
	}
	return nil
}

func getStatus(state int, err error) *HealthProp {
	return &HealthProp{
		StatusCode: state,
		Err:        err,
	}
}
