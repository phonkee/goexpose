package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/mcuadros/go-defaults"
	"github.com/phonkee/go-response"
	"github.com/phonkee/goexpose/domain"
	"github.com/phonkee/goexpose/internal/pubsub"
	"github.com/phonkee/goexpose/internal/tasks/registry"
	"github.com/phonkee/goexpose/internal/utils"
	"go.uber.org/zap"
	"net/http"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
	"strings"
	"time"
)

func init() {
	registry.RegisterTaskInitFunc("pubsub", PubSubTaskFactory)
}

// PubSubTaskFactory creates pubsub task from single task config
func PubSubTaskFactory(server domain.Server, tc *domain.TaskConfig, ec *domain.EndpointConfig) (_ []domain.Task, err error) {

	// first create config with default values
	config := PubSubConfig{
		Hub:    pubsub.NewPool(),
		Shared: true,
	}

	// set default values
	defaults.SetDefaults(&config)

	// unmarshal from config
	if err = json.Unmarshal(tc.Config, &config); err != nil {
		return
	}

	// group by Group, if not present generate uuid for task instance
	if config.Group = strings.TrimSpace(config.Group); config.Group == "" {
		config.Shared = false
		config.Group = uuid.NewString()
	}

	return []domain.Task{&config}, nil
}

// PubSubConfig serves as pubsub task. It is built on websockets.
// you connect to this websocket and then you can send/receive messages
// it has simple protocol:
//
//		{"publish": {"channel": "<channel>", "message":"<message>"}}
//		{"subscribe":{"channel": ["<channel>", "<channel>"]} // single or list...
//		{"unsubscribe": {"channel": ["<channel>", "<channel>"]} single or list
//		{"subscriptions": true}
//	 	{"message": "<message>", "channel": "<channel>"}
//
// message can contain template, and can use data that is configured in endpoint
type PubSubConfig struct {
	domain.BaseTask

	// Group to run multiple endpoints on single pubsub system
	Group                  string        `json:"group"`
	Shared                 bool          `json:"-"`
	WebsocketRetryDuration time.Duration `json:"websocket_retry_duration" default:"10s"`
	MaxQueueSize           int           `json:"max_queue_size" default:"32"`

	// Hub
	Hub pubsub.Pool `json:"-" default:"-"`
}

// Options creates options for new hub
func (p *PubSubConfig) Options() []pubsub.Option {
	return []pubsub.Option{
		pubsub.WithMaxQueueSize(p.MaxQueueSize),
	}
}

// Run websocket
func (p *PubSubConfig) Run(r *http.Request, vars map[string]interface{}) response.Response {
	if b, e := utils.RenderTextTemplate(p.Group, vars); e != nil {
		return response.Error(e)
	} else {
		p.Group = b
	}

	// first get pubsub hub, if not created it will be with given options (passing options to hub)
	hub, created, err := p.Hub.Get(p.Group, func() []pubsub.Option {
		return p.Options()
	})

	if created {
		zap.L().Debug("created new pubsub hub", zap.String("group", p.Group))
	}

	if err != nil {
		return response.Error(err)
	}

	// create session for this request
	sess, err := hub.NewSession()
	if err != nil {
		return response.Error(err)
	}

	// close after exit, this is important so we don't have any leaks
	defer func() {
		_ = sess.Close()
	}()

	// we need to have ResponseWriter, it's a bit hacky, but it works
	w, ok := r.Context().Value(domain.WriterContextKey).(http.ResponseWriter)
	if !ok {
		return response.Error(fmt.Errorf("no way"))
	}

	// create websocket client
	clientWS, err := websocket.Accept(w, r, nil)

	if err != nil {
		return response.Error(err)
	}

	defer clientWS.Close(websocket.StatusInternalError, "")

	var psr pubsub.Request

	// if we have query values (channel) we automatically subscribe

	for {
		// func for defer
		func() {
			ctx, cancel := context.WithTimeout(r.Context(), p.WebsocketRetryDuration)
			defer cancel()

			err = wsjson.Read(ctx, clientWS, &psr)
		}()

		// check what kind of error it is
		if err != nil {
			if err == context.DeadlineExceeded {
				continue
			}

			// TODO: check other errors

			zap.L().Error("error reading from websocket", zap.Error(err))
			continue
		}

		// TODO: first validate request data
		// psr.Validate()

		resp, err := psr.Execute(r.Context(), sess)
		if err != nil {
			zap.L().Error("error running request", zap.Error(err))
			continue
		}

		if err := WriteResponse(clientWS, r, resp); err != nil {
			zap.L().Error("error writing response", zap.Error(err))
			continue
		}
	}

	// TODO: what about this?
	// if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
	//		websocket.CloseStatus(err) == websocket.StatusGoingAway {
	//		return
	//	}
}

func WriteResponse(c *websocket.Conn, r *http.Request, resp *pubsub.Response) (err error) {
	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return c.Write(r.Context(), websocket.MessageText, b)
}
