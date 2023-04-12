package pubsub

import (
	"context"
	"github.com/phonkee/goexpose/domain"
	"go.uber.org/zap"
	"sync"
)

// Hub system that maintains sessions and channels
//
//go:generate mockery --name=Hub
type Hub interface {
	Close() error
	NewSession() (Session, error)

	// Subscribe creates session and subscribes to channels
	Subscribe(ctx context.Context, channels ...string) (Session, error)

	// Session returns session by id
	Session(id string) (Session, bool)

	// Publish message to all sessions
	Publish(ctx context.Context, message *domain.PubSubMessage) error

	// private
	closeSession(id string) error
}

func New(options ...Option) (Hub, error) {
	result := &hub{
		sessions:       make(map[string]Session),
		sessionsMutex:  &sync.RWMutex{},
		maxChannelSize: 32,
	}

	for _, option := range options {
		if err := option(result); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// implementation of Hub
type hub struct {
	maxChannelSize int // buffered channel
	sessions       map[string]Session
	sessionsMutex  *sync.RWMutex
}

func (p *hub) closeSession(id string) error {
	p.sessionsMutex.Lock()
	defer p.sessionsMutex.Unlock()

	delete(p.sessions, id)

	return nil
}

func (p *hub) Subscribe(ctx context.Context, channels ...string) (Session, error) {
	sess, err := p.NewSession()
	if err != nil {
		return nil, err
	}
	if err := sess.Subscribe(ctx, channels...); err != nil {
		return nil, err
	}
	return sess, nil
}

func (p *hub) Close() error {
	return nil
}

// NewSession creates new session, if that session will be closed, it will be removed from hub.
func (p *hub) NewSession() (Session, error) {

	s := newSession(p, &sessionSettings{
		maxChannelSize: p.maxChannelSize,
	})

	p.sessionsMutex.Lock()
	p.sessions[s.ID()] = s
	p.sessionsMutex.Unlock()

	return s, nil
}

// Publish message to sessions listeners
func (p *hub) Publish(ctx context.Context, message *domain.PubSubMessage) error {

	// returns error just initially, we publish to sessions in goroutine

	p.sessionsMutex.RLock()
	defer p.sessionsMutex.RUnlock()

	// call every session Publish.
	for _, ss := range p.sessions {
		go func(sess Session) {
			if err := sess.Publish(ctx, message); err != nil {
				zap.L().Error("publish error", zap.Error(err))
			}
		}(ss)
	}

	return nil
}

// Session returns session by id if available
func (p *hub) Session(id string) (Session, bool) {
	p.sessionsMutex.RLock()
	defer p.sessionsMutex.RUnlock()

	s, ok := p.sessions[id]
	return s, ok
}

// SessionIDs returns list of session ids
func (p *hub) SessionIDs() []string {
	p.sessionsMutex.RLock()
	defer p.sessionsMutex.RUnlock()

	result := make([]string, 0, len(p.sessions))
	for id := range p.sessions {
		result = append(result, id)
	}
	return result
}
