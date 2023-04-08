package domain

import "errors"

var (
	ErrInvalidSender     = errors.New("invalid sender")
	ErrMissingRecipients = errors.New("missing recipients")
	ErrInvalidRecipient  = errors.New("invalid recipient")
	ErrEmptySubject      = errors.New("empty subject")
)
