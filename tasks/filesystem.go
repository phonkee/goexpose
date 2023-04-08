package tasks

import (
	"encoding/base64"
	"encoding/json"
	"github.com/phonkee/go-response"
	"github.com/phonkee/goexpose"
	"github.com/phonkee/goexpose/domain"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	goexpose.RegisterTaskFactory("filesystem", FilesystemFactory)
}

/*
Filesystem task gives possibility to serve files.
*/

func NewFilesystemConfig() *FilesystemConfig {
	return &FilesystemConfig{}
}

type FilesystemConfig struct {
	File      string `json:"file"`
	Output    string `json:"output"`
	Directory string `json:"directory"`
	Index     bool   `json:"index"`
}

func (f *FilesystemConfig) Validate() (err error) {
	// cleanup strings
	f.File = strings.TrimSpace(f.File)
	f.Directory = strings.TrimSpace(f.Directory)
	return
}

/*
FilesystemTask

	serve single file
*/
type FilesystemTask struct {
	domain.BaseTask
	config *FilesystemConfig
}

/*
Run method for FilesystemTask
*/
func (f *FilesystemTask) Run(r *http.Request, data map[string]interface{}) response.Response {

	var (
		directory string
		err       error
		filename  string
		finfo     os.FileInfo
		output    string
	)

	// interpolate filename
	if filename, err = goexpose.Interpolate(f.config.File, data); err != nil {
		return response.Error(err)
	}

	// interpolate directory
	if directory, err = goexpose.Interpolate(f.config.Directory, data); err != nil {
		return response.Error(err)
	}

	full := filepath.Join(directory, filename)

	if finfo, err = os.Stat(full); err != nil {
		return response.NotFound()
	}

	// it's directory
	if finfo.IsDir() {
		if !f.config.Index {
			return response.NotFound()
		}

		var (
			items []os.FileInfo
			qr    *goexpose.Response
		)
		if items, err = ioutil.ReadDir(full); err != nil {
			return response.Error(err)
		}

		// prepare results
		results := make([]*goexpose.Response, len(items))
		for i, item := range items {
			qr = goexpose.NewResponse(http.StatusOK).StripStatusData()
			qr.Result(filepath.Join(full, item.Name())).AddValue("is_dir", item.IsDir())
			results[i] = qr
		}

		return response.Result(results)
	}

	var contents []byte
	if contents, err = ioutil.ReadFile(full); err != nil {
		return response.Error(err)
	}

	if output, err = goexpose.Interpolate(f.config.Output, data); err != nil {
		return response.Error(err).Status(http.StatusInternalServerError)
	}

	// raw body
	if strings.TrimSpace(strings.ToLower(output)) == "raw" {
		return response.HTML(string(contents))
	}

	b64content := base64.StdEncoding.EncodeToString(contents)
	_, ff := filepath.Split(full)

	return response.Result(b64content).Data("filename", ff)
}

// FilesystemFactory creates filesystem tasks
func FilesystemFactory(s domain.Server, tc *domain.TaskConfig, ec *domain.EndpointConfig) (result []domain.Task, err error) {

	config := NewFilesystemConfig()
	if err = json.Unmarshal(tc.Config, config); err != nil {
		return
	}

	if err = config.Validate(); err != nil {
		return
	}

	result = []domain.Task{&FilesystemTask{
		config: config,
	}}
	return
}