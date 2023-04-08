package tasks

import (
	"encoding/json"
	"fmt"
	"github.com/mcuadros/go-defaults"
	_ "github.com/mcuadros/go-defaults"
	"github.com/phonkee/goexpose"
	"github.com/phonkee/goexpose/domain"
	"gopkg.in/gomail.v2"
	_ "gopkg.in/gomail.v2"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"text/template"

	htmlTemplate "html/template"
	textTemplate "text/template"
)

// EmailTaskFactory creates email task from single task config
func EmailTaskFactory(server goexpose.Server, taskconfig *domain.TaskConfig, ec *domain.EndpointConfig) (tasks []domain.Tasker, err error) {

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
	if !config.Debug {
		if err = config.Connect(); err != nil {
			return
		}
	}

	return []domain.Tasker{config}, nil
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

// EmailTaskConfig represents email task config
type EmailTaskConfig struct {
	domain.BaseTask
	Smtp         EmailSmtpConfig        `json:"smtp"`
	Sender       string                 `json:"sender"`
	Recipients   []string               `json:"recipients"`
	Data         map[string]interface{} `json:"data"`
	Subject      string                 `json:"subject"`
	Body         string                 `json:"body"`
	BodyFilename string                 `json:"body_filename"`
	Debug        bool                   `json:"debug"`
	Html         bool                   `json:"html"`

	contentType     string                  `json:"-"`
	subjectTemplate domain.TemplateExecutor `json:"-"`
	bodyTemplate    domain.TemplateExecutor `json:"-"`
}

// compile templates
func (e *EmailTaskConfig) compile() (err error) {
	// check subject first
	if subject := strings.TrimSpace(e.Subject); subject == "" {
		return domain.ErrEmptySubject
	} else {
		if e.subjectTemplate, err = template.New(fmt.Sprintf("subject_%v", rand.Int())).Parse(subject); err != nil {
			return
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
			return fmt.Errorf("please provide either template or template_filename")
		}
		templateString = e.Body
	}

	// now parse appropriate template
	if e.Html {
		e.contentType = "text/html"
		if e.bodyTemplate, err = htmlTemplate.New("email").Parse(templateString); err != nil {
			return fmt.Errorf("%w: cannot parse html template", err)
		}
	} else {
		e.contentType = "text/plain"
		if e.bodyTemplate, err = textTemplate.New("email").Parse(templateString); err != nil {
			return fmt.Errorf("%w: cannot parse text template", err)
		}
	}

	return nil
}

// Validate configuration
func (e *EmailTaskConfig) Validate() error {
	if sender := strings.TrimSpace(e.Sender); sender == "" {
		return domain.ErrInvalidSender
	}

	if !e.Debug {
		if len(e.Recipients) == 0 {
			return domain.ErrMissingRecipients
		}
		for _, recipient := range e.Recipients {
			if strings.TrimSpace(recipient) == "" {
				return domain.ErrInvalidRecipient
			}
		}
	}

	return nil
}

// Run request
func (e *EmailTaskConfig) Run(r *http.Request, params map[string]interface{}) domain.Response {
	//TODO implement me
	panic("implement me")
}

// Connect to smtp server
func (e *EmailTaskConfig) Connect() error {
	if c, err := gomail.NewDialer(e.Smtp.Host, e.Smtp.Port, e.Smtp.Username, e.Smtp.Password).Dial(); err != nil {
		return err
	} else {
		return c.Close()
	}
}

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
