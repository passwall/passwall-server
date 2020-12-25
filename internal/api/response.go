package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"text/template"

	"github.com/go-playground/validator/v10"
	"github.com/passwall/passwall-server/model"
)

//ErrorResponseDTO represents error resposne
type ErrorResponseDTO struct {
	Code    int      `json:"code"`
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Errors  []string `json:"errors"`
}

type fieldError struct {
	err validator.FieldError
}

// RespondWithError ...
func RespondWithError(w http.ResponseWriter, code int, message string) {
	RespondWithJSON(w, code, ErrorResponseDTO{Code: code, Status: "Error", Message: message})
}

// RespondWithErrors ...
func RespondWithErrors(w http.ResponseWriter, code int, message string, errors []string) {
	RespondWithJSON(w, code, ErrorResponseDTO{Code: code, Status: "Error", Message: message, Errors: errors})
}

// RespondWithJSON write json
func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

//TODO if this function is not used, it should be deleted
// RespondWithHTML write html
func RespondWithHTML(w http.ResponseWriter, code int, payload interface{}) error {
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	t, err := template.ParseFiles("./store/template/email_confirmation_success.html")
	if err != nil {
		fmt.Fprintf(w, "Unable to load template")
	}

	// user := User{
	//               Id: 1,
	//               Name: "John Doe",
	//               Email: "johndoe@gmail.com",
	//               Phone: "000099999"
	//            }

	err = t.Execute(w, payload.(model.Response))

	if err != nil {
		return err
	}

	return nil
}

// GetErrors ...
func GetErrors(errs []validator.FieldError) []string {
	var arr []string
	for _, fe := range errs {
		arr = append(arr, (fieldError{fe}.String()))
	}
	return arr
}

func (q fieldError) String() string {
	var sb strings.Builder

	sb.WriteString("validation failed on field '" + q.err.Field() + "'")
	sb.WriteString(", condition: " + q.err.ActualTag())

	// Print condition parameters, e.g. oneof=red blue -> { red blue }
	if q.err.Param() != "" {
		sb.WriteString(" { " + q.err.Param() + " }")
	}

	if q.err.Value() != nil && q.err.Value() != "" {
		sb.WriteString(fmt.Sprintf(", actual: %v", q.err.Value()))
	}

	return sb.String()
}
