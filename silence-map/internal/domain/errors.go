package domain

import "errors"

var (
	ErrValidation            = errors.New("validation error")
	ErrNotFound              = errors.New("resource not found")
	ErrDuplicateConfirmation = errors.New("user already confirmed this report")
	ErrSelfConfirmation      = errors.New("report author cannot confirm their own report")
)

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return e.Field + ": " + e.Message
}

func (e *ValidationError) Is(target error) bool {
	return target == ErrValidation
}
