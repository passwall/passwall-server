package app

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"

	"github.com/go-playground/validator/v10"
)

// GetMD5Hash ...
func GetMD5Hash(text []byte) (string, error) {
	hasher := md5.New()
	if _, err := hasher.Write(text); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// RandomMD5Hash returns random md5 hash for unique conifrim links
func RandomMD5Hash() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	r, err := GetMD5Hash(b)
	if err != nil {
		return "", err
	}

	return r, nil
}

// PayloadValidator ...
func PayloadValidator(model interface{}) error {
	validate := validator.New()
	validateError := validate.Struct(model)
	if validateError != nil {
		//	errs := GetErrors(validateError.(validator.ValidationErrors))
		//	RespondWithErrors(w, http.StatusBadRequest, InvalidRequestPayload, errs)
		return validateError
	}
	return nil
}
