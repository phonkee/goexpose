package pubsub

// Option for new hub
type Option func(*hub) error

// WithMaxQueueSize sets maximum queue size, when full, messages will be dropped
func WithMaxQueueSize(size int) Option {
	return func(p *hub) error {
		p.maxChannelSize = size
		return nil
	}
}
