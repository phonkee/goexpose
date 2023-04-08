package tasks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/mcuadros/go-defaults"
	_ "github.com/mcuadros/go-defaults"
	"github.com/phonkee/go-response"
	"github.com/phonkee/goexpose/domain"
	"github.com/phonkee/goexpose/tasks/registry"
	"gopkg.in/gomail.v2"
	_ "gopkg.in/gomail.v2"
	"math/rand"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"text/template"

	_ "embed"
	htmlTemplate "html/template"
	textTemplate "text/template"
)

func init() {
	registry.RegisterTaskInitFunc("email", EmailTaskFactory)
}

var (
	//go:embed templates/debug.html
	debugTemplateString string
	debugTemplate       *textTemplate.Template
)

func init() {
	// pre-compile debug template
	debugTemplate = textTemplate.Must(textTemplate.New("debug").Parse(debugTemplateString))
}

// EmailTaskFactory creates email task from single task config
func EmailTaskFactory(server domain.Server, taskconfig *domain.TaskConfig, ec *domain.EndpointConfig) (tasks []domain.Task, err error) {

	// first create config with default values
	config := NewEmailTaskConfig()

	// unmarshal from config
	if err = json.Unmarshal(taskconfig.Config, config); err != nil {
		return
	}

	// now validate config
	if err = config.Validate(); err != nil {
		return
	}

	// compile templates
	if err = config.compile(); err != nil {
		return
	}

	// if we are not in debug mode, try to connect to smtp server
	if !config.Debug && !config.DisableConnectCheck {
		if err = config.Connect(); err != nil {
			return
		}
	}

	return []domain.Task{config}, nil
}

// NewEmailTaskConfig instantiates new email task config with default values
func NewEmailTaskConfig() *EmailTaskConfig {
	result := EmailTaskConfig{}
	defaults.SetDefaults(&result)
	return &result
}

type EmailSmtpConfig struct {
	Host     string `json:"host" default:"localhost"`
	Port     int    `json:"port" default:"25"`
	Username string `json:"username"`
	Password string `json:"password"`
	Cache    bool   `json:"cache" default:"false"` // not working currently
}

func (e *EmailSmtpConfig) Validate() error {
	if e == nil {
		return domain.ErrMissingSmtp
	}
	if host := strings.TrimSpace(e.Host); host == "" {
		return fmt.Errorf("%w: %v", domain.ErrInvalidSmtp, "host")
	}

	return nil
}

// EmailTaskConfig represents email task config
type EmailTaskConfig struct {
	domain.BaseTask
	Smtp                *EmailSmtpConfig       `json:"smtp"`
	Sender              string                 `json:"sender"`
	Recipients          []string               `json:"recipients"`
	Data                map[string]interface{} `json:"data"`
	Subject             string                 `json:"subject"`
	Body                string                 `json:"body"`
	BodyFilename        string                 `json:"body_filename"`
	Debug               bool                   `json:"debug"`
	Html                bool                   `json:"html"`
	DisableConnectCheck bool                   `json:"disable_connect_check"` // please use sparingly
	// private parts (do not look)
	contentType     string
	subjectTemplate domain.TemplateExecutor
	bodyTemplate    domain.TemplateExecutor
}

// compile templates
func (e *EmailTaskConfig) compile() (err error) {
	// check subject first
	if subject := strings.TrimSpace(e.Subject); subject == "" {
		return domain.ErrEmptySubject
	} else {
		if e.subjectTemplate, err = template.New(fmt.Sprintf("subject_%v", rand.Int())).Parse(subject); err != nil {
			return fmt.Errorf("%w: subject: %v", domain.ErrInvalidTemplate, err)
		}
	}

	// now check body and body filename
	var templateString string
	if e.BodyFilename != "" {
		b, err := os.ReadFile(e.BodyFilename)
		if err != nil {
			return fmt.Errorf("%w: cannot read template file %v", err, e.BodyFilename)
		}
		templateString = string(b)
	} else {
		if e.Body == "" {
			return domain.ErrBodyMissing
		}
		templateString = e.Body
	}

	// now parse appropriate template
	if e.Html {
		e.contentType = "text/html"
		if e.bodyTemplate, err = htmlTemplate.New("email").Parse(templateString); err != nil {
			return fmt.Errorf("%w: body: %v", domain.ErrInvalidTemplate, err)
		}
	} else {
		e.contentType = "text/plain"
		if e.bodyTemplate, err = textTemplate.New("email").Parse(templateString); err != nil {
			return fmt.Errorf("%w: body: %v", domain.ErrInvalidTemplate, err)
		}
	}

	return nil
}

// Validate configuration
func (e *EmailTaskConfig) Validate() error {

	// check for smtp
	if !e.Debug {
		if err := e.Smtp.Validate(); err != nil {
			return err
		}
	}

	// sender is always required
	if sender := strings.TrimSpace(e.Sender); sender == "" {
		return domain.ErrInvalidSender
	}

	// if not debug, recipients must be set
	if !e.Debug {
		if len(e.Recipients) == 0 {
			return domain.ErrMissingRecipients
		}
		recipients := make([]string, 0, len(e.Recipients))
		for _, recipient := range e.Recipients {
			if strings.TrimSpace(recipient) == "" {
				return domain.ErrInvalidRecipient
			} else {
				if address, err := mail.ParseAddress(recipient); err != nil {
					return fmt.Errorf("%w: %v", domain.ErrInvalidRecipient, recipient)
				} else {
					recipients = append(recipients, address.Address)
				}
			}
		}
		e.Recipients = recipients
	}

	return nil
}

// Run request
func (e *EmailTaskConfig) Run(r *http.Request, params map[string]interface{}) response.Response {
	var (
		data map[string]interface{}
		err  error
		m    *gomail.Message
	)
	// get all necessary data
	if data, err = e.GetData(r, params); err != nil {
		return response.Error(err)
	}

	// get message
	if m, err = e.GetMessage(data); err != nil {
		return response.Error(err)
	}

	// check for debug
	if e.Debug {
		var result bytes.Buffer

		messageBuffer := bytes.Buffer{}
		if _, err := m.WriteTo(&messageBuffer); err != nil {
			return response.Error(err)
		}

		// prepare response
		if err := debugTemplate.Execute(&result, map[string]interface{}{
			"data":    data,
			"message": messageBuffer.String(),
		}); err != nil {
			return response.Error(err)
		}

		return response.HTML(result.String())
	}

	// send actual email
	if err = e.Send(m); err != nil {
		return response.Error(err)
	}

	return nil
}

// Connect to smtp server
func (e *EmailTaskConfig) Connect() error {
	if c, err := gomail.NewDialer(e.Smtp.Host, e.Smtp.Port, e.Smtp.Username, e.Smtp.Password).Dial(); err != nil {
		return err
	} else {
		return c.Close()
	}
}

// Send multiple messages
func (e *EmailTaskConfig) Send(m ...*gomail.Message) error {
	return gomail.NewDialer(e.Smtp.Host, e.Smtp.Port, e.Smtp.Username, e.Smtp.Password).DialAndSend(m...)
}

// GetData returns data for both subject and body templates
func (e *EmailTaskConfig) GetData(r *http.Request, params map[string]interface{}) (data map[string]interface{}, err error) {
	data = map[string]interface{}{}

	// first data from config, that will be overridden by params
	for k, v := range e.Data {
		data[k] = v
	}
	for k, v := range params {
		data[k] = v
	}

	return data, nil
}

// GetMessage returns smtp message for given data
func (e *EmailTaskConfig) GetMessage(data map[string]interface{}) (m *gomail.Message, err error) {
	var (
		subject bytes.Buffer
		body    bytes.Buffer
	)

	// first subject
	if err = e.subjectTemplate.Execute(&subject, data); err != nil {
		return
	}

	// now body
	if err = e.bodyTemplate.Execute(&body, data); err != nil {
		return
	}

	// now create message
	m = gomail.NewMessage()
	m.SetHeader("From", e.Sender)
	m.SetHeader("To", e.Recipients...)
	m.SetHeader("Subject", subject.String())
	m.SetBody(e.contentType, body.String())

	return
}
