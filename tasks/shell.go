package tasks

import (
	"encoding/json"
	"fmt"
	"github.com/phonkee/goexpose"
	"github.com/phonkee/goexpose/domain"
	"net/http"
	"os/exec"
	"strings"
)

func init() {
	goexpose.RegisterTaskFactory("shell", ShellTaskFactory)
}

// ShellTaskConfig for shell task
type ShellTaskConfig struct {
	// Custom environment variables
	Env               map[string]string         `json:"env"`
	Shell             string                    `json:"shell"`
	Commands          []*ShellTaskConfigCommand `json:"commands"`
	SingleResult      *int                      `json:"single_result"`
	singleResultIndex int
}

/*
Validate validates config
*/
func (s *ShellTaskConfig) Validate() (err error) {
	if len(s.Commands) == 0 {
		return errors.New("please provide at least one command")
	}
	for _, c := range s.Commands {
		if err = c.Validate(); err != nil {
			return
		}
	}
	if s.SingleResult != nil {
		s.singleResultIndex = *s.SingleResult
		if s.singleResultIndex > len(s.Commands)-1 {
			return errors.New("single_result out of bounds")
		}
	} else {
		s.singleResultIndex = -1
	}
	return
}

type ShellTaskConfigCommand struct {
	Command       string `json:"command"`
	Chdir         string `json:"chdir"`
	Format        string `json:"format"`
	ReturnCommand bool   `json:"return_command"`
}

func (s *ShellTaskConfigCommand) Validate() (err error) {
	if s.Format, err = goexpose.VerifyFormat(s.Format); err != nil {
		return
	}
	return
}

func NewShellTaskConfig() *ShellTaskConfig {
	return &ShellTaskConfig{
		Shell: "/bin/sh",
		Env:   map[string]string{},
	}
}

/*
Factory for ShellTask
*/
func ShellTaskFactory(server goexpose.Server, taskconfig *domain.TaskConfig, ec *domain.EndpointConfig) (tasks []domain.Task, err error) {
	config := NewShellTaskConfig()
	if err = json.Unmarshal(taskconfig.Config, config); err != nil {
		return
	}

	if err = config.Validate(); err != nil {
		return
	}

	tasks = []domain.Task{&ShellTask{
		Config: config,
	}}
	return
}

/*
ShellTask runs shell commands
*/
type ShellTask struct {
	Task

	// config
	Config *ShellTaskConfig
}

/*
Run method for shell task
Run all commands and return results
*/
func (s *ShellTask) Run(r *http.Request, data map[string]interface{}) (response *goexpose.Response) {

	var results []*goexpose.Response

	response = goexpose.NewResponse(http.StatusOK)

	// run all commands
	for _, command := range s.Config.Commands {

		// strip status data from response
		cmdresp := goexpose.NewResponse(http.StatusOK).StripStatusData()

		var (
			b            string
			e            error
			finalCommand string
			cmd          *exec.Cmd
		)
		if b, e = goexpose.Interpolate(command.Command, data); e != nil {
			cmdresp.Error(e)
			goto Append
		}

		finalCommand = b

		// show command in result
		if command.ReturnCommand {
			cmdresp.AddValue("command", finalCommand)
		}

		// run command
		cmd = exec.Command(s.Config.Shell, "-c", finalCommand)

		// change directory if needed
		if command.Chdir != "" {
			cmd.Dir = command.Chdir
		}

		// add env vars
		for k, v := range s.Config.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}

		// get output
		if out, err := cmd.Output(); err != nil {
			cmdresp.Error(err)
			goto Append
		} else {
			// format out
			if re, f, e := goexpose.Format(string(strings.TrimSpace(string(out))), command.Format); e == nil {
				cmdresp.Result(re).AddValue("format", f)
			} else {
				cmdresp.Error(e)
			}
			goto Append
		}

	Append:
		results = append(results, cmdresp.StripStatusData())
	}

	// single result
	if s.Config.singleResultIndex != -1 {
		response.Result(results[s.Config.singleResultIndex])
	} else {
		response.Result(results)
	}

	return
}