package api

import (
	"net/http"

	"github.com/passwall/passwall-server/internal/storage"
)

var (
	//Port representd a server port
	Port = "3625"
	//ServerAddress represents a server addres
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
		checkResult := Services{
			API:      getStatus(http.StatusOK, nil),
			Database: getStatus(http.StatusOK, nil),
		}

		RespondWithJSON(w, checkResult.Database.StatusCode, checkResult)
	}
}

func checkEndPoint(url string) error {
	if _, err := http.Get(url); err != nil {
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
