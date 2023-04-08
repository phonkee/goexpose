package domain

import "context"

// Server interface
type Server interface {

	// Run runs server
	Run(ctx context.Context) error
}
