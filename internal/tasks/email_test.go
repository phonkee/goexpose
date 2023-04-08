package tasks

import (
	_ "github.com/mocktools/go-smtp-mock/v2"
	smtpmock "github.com/mocktools/go-smtp-mock/v2"
	"github.com/phonkee/goexpose/domain"
	"github.com/phonkee/goexpose/domain/mocks"
	"github.com/stretchr/testify/assert"
	"testing"
)

const (
	usedPort = 1034
)

func TestEmailTaskConfig_Validate(t *testing.T) {

	validSmtp := func(fn ...func(config *EmailSmtpConfig)) *EmailSmtpConfig {
		result := EmailSmtpConfig{
			Host:     "localhost",
			Port:     usedPort,
			Username: "",
			Password: "",
		}
		for _, f := range fn {
			f(&result)
		}
		return &result
	}

	t.Run("invalid data", func(t *testing.T) {
		for _, item := range []struct {
			data        map[string]interface{}
			errContains string
		}{
			{ // missing smtp
				data:        map[string]interface{}{},
				errContains: domain.ErrMissingSmtp.Error(),
			},
			{ // invalid sender
				data: map[string]interface{}{
					"sender": "    ",
					"smtp":   validSmtp(),
				},
				errContains: domain.ErrInvalidSender.Error(),
			},
			{ // missing recipients in non debug mode
				data: map[string]interface{}{
					"sender": "hello@phonkee.eu",
					"smtp":   validSmtp(),
				},
				errContains: domain.ErrMissingRecipients.Error(),
			},
			{
				data: map[string]interface{}{
					"sender":     "hello@phonkee.eu",
					"recipients": []string{"   "},
					"smtp":       validSmtp(),
				},
				errContains: domain.ErrInvalidRecipient.Error(),
			},
			{ // no need to have recipients when debug is set
				data: map[string]interface{}{
					"sender": "hello@phonkee.eu",
					"debug":  true,
				},
				errContains: domain.ErrEmptySubject.Error(),
			},
			{ // smtp host is blank
				data: map[string]interface{}{
					"sender": "hello@phonkee.eu",
					"smtp": validSmtp(func(c *EmailSmtpConfig) {
						c.Host = ""
					}),
				},
				errContains: domain.ErrInvalidSmtp.Error(),
			},
		} {
			srv := mocks.NewServer(t)
			_, err := EmailTaskFactory(srv, taskConfigFrom(t, item.data), &domain.EndpointConfig{})
			assert.Contains(t, err.Error(), item.errContains)
		}
	})

	t.Run("invalid templates", func(t *testing.T) {
		server := smtpmock.New(smtpmock.ConfigurationAttr{
			PortNumber:        usedPort,
			LogToStdout:       true,
			LogServerActivity: true,
			HostAddress:       "localhost",
		})
		assert.NoError(t, server.Start())

		for _, item := range []struct {
			subject     string
			body        string
			errContains []string
		}{
			{
				subject:     "{{ .invalid",
				errContains: []string{domain.ErrInvalidTemplate.Error(), "subject"},
			},
			{
				subject:     "{{ .invalid }",
				errContains: []string{domain.ErrInvalidTemplate.Error(), "subject"},
			},
			{
				subject:     "{{ .valid }}",
				errContains: []string{domain.ErrBodyMissing.Error()},
			},
		} {
			data := map[string]interface{}{
				"sender":     "hello@phonkee.eu",
				"recipients": []string{"phonkee@phonkee.eu"},
				"smtp":       validSmtp(),
				"subject":    item.subject,
				"body":       item.body,
			}
			srv := mocks.NewServer(t)
			_, err := EmailTaskFactory(srv, taskConfigFrom(t, data), &domain.EndpointConfig{})
			assert.NotNil(t, err)
			for _, c := range item.errContains {
				assert.Contains(t, err.Error(), c)
			}
		}
	})
}
