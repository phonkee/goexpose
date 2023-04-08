package domain

import "errors"

var (
	ErrInvalidSender     = errors.New("invalid sender")
	ErrMissingRecipients = errors.New("missing recipients")
	ErrInvalidRecipient  = errors.New("invalid recipient")
	ErrEmptySubject      = errors.New("empty subject")
	ErrMissingSmtp       = errors.New("missing smtp")
	ErrInvalidSmtp       = errors.New("invalid smtp")
	ErrInvalidTemplate   = errors.New("invalid template")
	ErrBodyMissing       = errors.New("please provide either body or body_filename")
	ErrInvalidURL        = errors.New("invalid url")
)
