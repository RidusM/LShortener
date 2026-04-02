package entity

import "errors"

var (
	ErrDataNotFound       = errors.New("data not found")
	ErrInvalidData        = errors.New("invalid data")
	ErrConflictingData    = errors.New("conflicting data")
	ErrURLNotFound        = errors.New("url not found")
	ErrURLExpired         = errors.New("url expired")
	ErrURLInactive        = errors.New("url inactive")
	ErrAliasAlreadyExists = errors.New("alias already exists")
	ErrInvalidURL         = errors.New("invalid url format")
)
