package resourcecore

import "errors"

type ErrorCode string

const (
	InvalidArgument        ErrorCode = "INVALID_ARGUMENT"
	NotFound               ErrorCode = "NOT_FOUND"
	Duplicate              ErrorCode = "DUPLICATE"
	InvalidReference       ErrorCode = "INVALID_REFERENCE"
	Validation             ErrorCode = "VALIDATION"
	Integrity              ErrorCode = "INTEGRITY"
	IdentityConflict       ErrorCode = "IDENTITY_CONFLICT"
	InvalidLifecycle       ErrorCode = "INVALID_LIFECYCLE"
	ReactivationImpossible ErrorCode = "REACTIVATION_IMPOSSIBLE"
	InvalidCatalog         ErrorCode = "INVALID_CATALOG"
	InUse                  ErrorCode = "IN_USE"
	ImmutableCode          ErrorCode = "IMMUTABLE_CODE"
	Conflict               ErrorCode = "CONFLICT"
	Unavailable            ErrorCode = "UNAVAILABLE"
	Internal               ErrorCode = "INTERNAL"
)

type Error struct {
	code    ErrorCode
	message string
}

func (e Error) Error() string   { return e.message }
func (e Error) Code() ErrorCode { return e.code }

func NewError(code ErrorCode, message string) Error { return Error{code: code, message: message} }

func Code(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var e Error
	if errors.As(err, &e) {
		return e.code
	}
	return Internal
}

func IsCode(err error, code ErrorCode) bool { return Code(err) == code }
