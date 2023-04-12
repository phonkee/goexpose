package pubsub

import (
	"context"
	"github.com/phonkee/goexpose/domain"
	"reflect"
)

type RequestExecutor interface {
	Execute(ctx context.Context, session Session) (response *Response, err error)
}

// Request is protocol for websocket communication from client
// it knows how to execute itself, it uses reflection to find first non nil field and execute it.
// Each request should implement RequestExecutor
type Request struct {
	Publish       *RequestPublish       `json:"publish,omitempty"`
	Subscribe     *RequestSubscribe     `json:"subscribe,omitempty"`
	UnSubscribe   *RequestUnSubscribe   `json:"unsubscribe,omitempty"`
	Subscriptions *RequestSubscriptions `json:"subscriptions,omitempty"`
}

// Execute is generic execute uses reflection to execute request
// it walks on all fields and executes first non nil field and has Execute method
func (p *Request) Execute(ctx context.Context, session Session) (response *Response, err error) {
	val := reflect.ValueOf(p)

	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// iterate over fields, and check if it is RequestExecutor and not nil
	// then execute it
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i).Interface()
		if exec, ok := field.(RequestExecutor); ok {
			if !val.Field(i).IsNil() {
				return exec.Execute(ctx, session)
			}
		}
	}

	return nil, domain.ErrPubSubInvalidRequest
}

// RequestPublish is request to publish message to channel
type RequestPublish struct {
	Channel domain.JsonStringSlice `json:"channel"`
	Message domain.JsonBytes       `json:"message"`
}

func (p *RequestPublish) Execute(ctx context.Context, session Session) (response *Response, err error) {
	err = session.Publish(ctx, &domain.PubSubMessage{
		Channel: p.Channel,
		Body:    p.Message,
	})
	var errString string
	if err != nil {
		errString = err.Error()
	}

	return &Response{
		Simple: &ResponseSimple{
			Error: errString,
		},
	}, err
}

type RequestSubscribe struct {
	Channel domain.JsonStringSlice `json:"channel,omitempty"`
}

func (p *RequestSubscribe) Execute(ctx context.Context, session Session) (response *Response, err error) {
	err = session.Subscribe(ctx, p.Channel...)
	var errString string
	if err != nil {
		errString = err.Error()
	}
	return &Response{
		Simple: &ResponseSimple{
			Error: errString,
		},
	}, err
}

type RequestUnSubscribe struct {
	Channel domain.JsonStringSlice `json:"channel,omitempty"`
}

func (p *RequestUnSubscribe) Execute(ctx context.Context, session Session) (response *Response, err error) {
	err = session.UnSubscribe(ctx, p.Channel...)
	var errString string
	if err != nil {
		errString = err.Error()
	}
	return &Response{
		Simple: &ResponseSimple{
			Error: errString,
		},
	}, err
}

type RequestSubscriptions struct {
}

func (p *RequestSubscriptions) Execute(ctx context.Context, session Session) (response *Response, err error) {
	return &Response{
		Subscriptions: &ResponseSubscriptions{
			Channels: session.Subscriptions(ctx),
		},
	}, nil
}
