package app

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"

	"github.com/go-playground/validator/v10"
)

// GetMD5Hash ...
func GetMD5Hash(text []byte) string {
	hasher := md5.New()
	hasher.Write(text)
	return hex.EncodeToString(hasher.Sum(nil))
}

// RandomMD5Hash returns random md5 hash for unique conifrim links
func RandomMD5Hash() string {
	b := make([]byte, 16)
	rand.Read(b)
	return GetMD5Hash(b)
}

// PayloadValidator ...
func PayloadValidator(model interface{}) error {
	if validateError := validator.New().Struct(model); validateError != nil {
		//	errs := GetErrors(validateError.(validator.ValidationErrors))
		//	RespondWithErrors(w, http.StatusBadRequest, InvalidRequestPayload, errs)
		return validateError
	}
	return nil
}
