package pubsub

// Response is protocol for websocket communication from server
type Response struct {
	Simple        *ResponseSimple        `json:"simple,omitempty"`
	Subscriptions *ResponseSubscriptions `json:"subscriptions,omitempty"`
}

type ResponseSimple struct {
	Error string `json:"error,omitempty"`
}

type ResponseSubscriptions struct {
	ResponseSimple
	Channels []string `json:"channels,omitempty"`
}
