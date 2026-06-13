package validator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

// FieldError represents a single field validation failure with a human-readable message.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors is returned when one or more fields fail validation.
type ValidationErrors struct {
	Fields []FieldError
}

func (e *ValidationErrors) Error() string {
	msgs := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		msgs = append(msgs, fmt.Sprintf("%s: %s", f.Field, f.Message))
	}
	return strings.Join(msgs, "; ")
}

// validate is the package-level validator instance, created once.
var validate *validator.Validate

var (
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	phoneRegex    = regexp.MustCompile(`^\+`)
)

func init() {
	validate = validator.New(validator.WithRequiredStructEnabled())

	// username: only Latin letters, digits, dot, dash, underscore
	_ = validate.RegisterValidation("username", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		return usernameRegex.MatchString(val)
	})

	// phone: if present, must start with '+'
	_ = validate.RegisterValidation("phone", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		if val == "" {
			return true
		}
		return phoneRegex.MatchString(val)
	})
}

// Validate runs structural validation on v and returns a *ValidationErrors on failure.
// Returns nil when all constraints pass.
func Validate(v any) error {
	err := validate.Struct(v)
	if err == nil {
		return nil
	}

	validationErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return err
	}

	fields := make([]FieldError, 0, len(validationErrs))
	for _, fe := range validationErrs {
		fields = append(fields, FieldError{
			Field:   fieldName(fe),
			Message: messageForTag(fe),
		})
	}

	return &ValidationErrors{Fields: fields}
}

// fieldName returns the lowercased struct field name for use in error responses.
func fieldName(fe validator.FieldError) string {
	return strings.ToLower(fe.Field())
}

// messageForTag maps validator tags to human-readable error messages.
func messageForTag(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", strings.ToLower(fe.Field()))
	case "email":
		return "must be a valid email address"
	case "min":
		return fmt.Sprintf("must be at least %s characters", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s characters", fe.Param())
	case "len":
		return fmt.Sprintf("must be exactly %s characters", fe.Param())
	case "oneof":
		return fmt.Sprintf("must be one of: %s", fe.Param())
	case "url":
		return "must be a valid URL"
	case "uuid":
		return "must be a valid UUID"
	case "username":
		return "must contain only letters, digits, dots, dashes, or underscores"
	case "phone":
		return "must start with +"
	default:
		return fmt.Sprintf("failed on '%s' validation", fe.Tag())
	}
}
