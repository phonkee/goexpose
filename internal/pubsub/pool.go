package pubsub

import (
	"sync"
)

// Pool contains multiple hub instances
//
//go:generate mockery --name=Pool
type Pool interface {
	Close(id string) error
	Get(id string, opts ...func() []Option) (_ Hub, created bool, err error)
}

func NewPool() Pool {
	return &pool{
		hubs:      make(map[string]Hub),
		hubsMutex: &sync.RWMutex{},
	}
}

type pool struct {
	hubs      map[string]Hub
	hubsMutex *sync.RWMutex
}

// Close single hub
func (h *pool) Close(id string) error {
	h.hubsMutex.Lock()
	defer h.hubsMutex.Unlock()

	// TODO: some shutdown maybe
	delete(h.hubs, id)

	return nil
}

func (h *pool) Get(id string, opts ...func() []Option) (_ Hub, created bool, err error) {
	h.hubsMutex.RLock()
	hh, ok := h.hubs[id]
	h.hubsMutex.RUnlock()

	if !ok {
		created = true
		options := make([]Option, 0)
		for _, opt := range opts {
			options = append(options, opt()...)
		}

		// new hub with options
		hh, err = New(options...)

		if err != nil {
			return nil, true, err
		}
	}

	if created {
		h.hubsMutex.Lock()
		h.hubs[id] = hh
		h.hubsMutex.Unlock()
	}

	return hh, created, nil
}
