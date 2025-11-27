package middleware

import (
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// Helper function used inside handlers:
func ValidateStruct(s interface{}) error {
	return validate.Struct(s)
}
