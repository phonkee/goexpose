package pubsub

import (
	"context"
	"github.com/google/uuid"
	"github.com/phonkee/goexpose/domain"
	"go.uber.org/zap"
	"sync"
)

//go:generate mockery --name=Session
type Session interface {
	Close() error
	ID() string
	Match(what string) (string, bool)
	Subscribe(ctx context.Context, channels ...string) error
	UnSubscribe(ctx context.Context, channels ...string) error
	Subscriptions(ctx context.Context) []string
	Publish(ctx context.Context, message *domain.PubSubMessage) error
	// Receive receives message from channel
	Receive(ctx context.Context) (message *domain.PubSubMessage, err error)
}

func newSession(parent *hub, settings *sessionSettings) *session {
	return settings.new(parent)
}

type sessionSettings struct {
	maxChannelSize int
}

// New creates session from settings
func (s *sessionSettings) new(parent *hub) *session {
	result := &session{
		closeChan:         make(chan struct{}, 1),
		closed:            false,
		id:                uuid.NewString(),
		subscriptions:     make(map[string]*subscription, 0),
		subscriptionMutex: &sync.RWMutex{},
		parent:            parent,
	}
	if s == nil {
		result.dataChan = make(chan data, 32)
	} else {
		result.dataChan = make(chan data, s.maxChannelSize)
	}

	return result
}

type session struct {
	id                string
	subscriptions     map[string]*subscription
	subscriptionMutex *sync.RWMutex
	parent            *hub
	closeChan         chan struct{}
	closed            bool
	dataChan          chan data
}

type data struct {
	channel string
	message *domain.PubSubMessage
	matched string
}

func (s *session) ID() string {
	return s.id
}

// Close all connections and channels, very important!
func (s *session) Close() (err error) {
	s.closed = true
	s.closeChan <- struct{}{}
	close(s.dataChan)

	// close in parent and disconnect for gc
	if s.parent != nil {
		err = s.parent.closeSession(s.id)
		s.parent = nil
	}

	return
}

func (s *session) Publish(ctx context.Context, message *domain.PubSubMessage) error {
	// error is returned only in critical case, all is done async way

	go func() {
		for _, channel := range message.Channel {
			if matched, ok := s.Match(channel); ok {
				// send message to channel
				zap.L().Debug("publishing message", zap.String("channel", channel), zap.String("subscriber", matched))

				// now we need to check if channel is full
				// if it is full, we cannot do anything and just warn in logs
				if len(s.dataChan) == cap(s.dataChan) {
					zap.L().Warn("channel full", zap.String("channel", channel), zap.Int("capacity", cap(s.dataChan)), zap.String("subscriber", matched))
					return
				}

				// we can safely add message
				s.dataChan <- data{message: message, matched: matched}
				return
			}
		}
	}()

	return nil
}

func (s *session) Match(what string) (string, bool) {
	s.subscriptionMutex.RLock()
	defer s.subscriptionMutex.RUnlock()

	for _, subs := range s.subscriptions {
		if subs.Matches(what) {
			return subs.Name(), true
		}
	}

	return "", false
}

func (s *session) Receive(ctx context.Context) (message *domain.PubSubMessage, err error) {
	if s.closed {
		return nil, domain.ErrPubSubClosed
	}

outer:
	for {
		select {
		case <-s.closeChan:
			break outer
		case c, ok := <-s.dataChan:
			if !ok {
				break outer
			}
			return c.message, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, domain.ErrPubSubClosed
}

func (s *session) Subscribe(ctx context.Context, channels ...string) error {
	s.subscriptionMutex.Lock()
	defer s.subscriptionMutex.Unlock()

	for _, channel := range channels {
		subs, err := parseSubscription(channel)
		if err != nil {
			return err
		}
		s.subscriptions[channel] = subs
	}

	return nil
}

func (s *session) UnSubscribe(ctx context.Context, channels ...string) error {
	s.subscriptionMutex.Lock()
	defer s.subscriptionMutex.Unlock()

	for _, channel := range channels {
		delete(s.subscriptions, channel)
	}

	return nil
}

func (s *session) Subscriptions(ctx context.Context) []string {
	s.subscriptionMutex.Lock()
	defer s.subscriptionMutex.Unlock()

	result := make([]string, 0, len(s.subscriptions))

	for name := range s.subscriptions {
		result = append(result, name)
	}

	return result
}
