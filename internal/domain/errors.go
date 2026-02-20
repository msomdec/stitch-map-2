package domain

import "errors"

var (
	ErrNotFound                = errors.New("not found")
	ErrDuplicateEmail          = errors.New("email already exists")
	ErrDuplicateAbbreviation   = errors.New("abbreviation already exists")
	ErrReservedAbbreviation    = errors.New("abbreviation is reserved")
	ErrUnauthorized            = errors.New("unauthorized")
	ErrInvalidInput            = errors.New("invalid input")
)
