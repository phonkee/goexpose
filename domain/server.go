package domain

import "context"

// Server no comments yet (in flux)
type Server interface {
	Run(ctx context.Context) error
	GetRoutes(ignored []string) (routes []*Route, err error)
	GetVersion() string
}
