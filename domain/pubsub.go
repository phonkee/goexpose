package domain

// PubSubMessage holds message and channel
type PubSubMessage struct {
	Channel JsonStringSlice
	Body    JsonBytes
}
