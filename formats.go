package goexpose

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// format function
type FormatFunc func(string) (interface{}, error)

var (
	formats          = map[string]FormatFunc{}
	formatslock      = &sync.RWMutex{}
	formatsdelimiter = "|"
)

func init() {
	// register formats
	RegisterFormat("json", FormatJSON)
	RegisterFormat("jsonlines", FormatJSONLines)
	RegisterFormat("lines", FormatLines)
	RegisterFormat("text", FormatText)
}

/*
Register format function
*/
func RegisterFormat(id string, fn FormatFunc) {
	formatslock.Lock()
	defer formatslock.Unlock()

	if _, ok := formats[id]; ok {
		panic(fmt.Sprintf("format %s already registered", id))
	}
	formats[id] = fn
}

/*
Verify given format

format can be multiple formats separated by "|". if text is not found in format
it is automatically added.
*/
func VerifyFormat(format string) (result string, err error) {
	formatslock.RLock()
	defer formatslock.RUnlock()

	ff := []string{}
	foundtext := false
	for _, part := range strings.Split(format, formatsdelimiter) {
		if part == "" {
			continue
		}
		if _, ok := formats[part]; !ok {
			err = fmt.Errorf("format %s unknown", part)
			return
		}
		ff = append(ff, part)
		if part == "text" {
			foundtext = true
		}
	}

	if !foundtext {
		ff = append(ff, "text")
	}

	result = strings.Join(ff, formatsdelimiter)
	return
}

/*
Formats body
*/
func Format(body string, f string) (result interface{}, format string, err error) {
	formatslock.RLock()
	defer formatslock.RUnlock()

	var (
		formatter FormatFunc
		ok        bool
	)
	for _, part := range strings.Split(f, formatsdelimiter) {
		if formatter, ok = formats[part]; !ok {
			err = fmt.Errorf("format %s unknown", part)
			return
		}
		result, err = formatter(body)
		format = part

		if err == nil {
			return
		}
	}

	return
}

/*
 */
func HasFormat(format, id string) bool {
	for _, part := range strings.Split(format, formatsdelimiter) {
		if part == id {
			return true
		}
	}
	return false
}

func AddFormat(format, id string) (result string) {

	for _, part := range strings.Split(format, formatsdelimiter) {
		if part == id {
			return format
		}
	}

	adder := id
	if format != "" {
		adder = adder + formatsdelimiter
	}

	return adder + format
}

// FormatJSON Formats body as json (map[string]interface{})
func FormatJSON(body string) (result interface{}, err error) {
	data := map[string]interface{}{}
	if err = json.Unmarshal([]byte(body), &data); err != nil {
		return
	}

	result = data
	return
}

// FormatJSONLines Formats body as json lines
func FormatJSONLines(body string) (result interface{}, err error) {

	r := []map[string]interface{}{}

	for _, line := range strings.Split(body, "\n") {
		data := map[string]interface{}{}

		if err = json.Unmarshal([]byte(line), &data); err != nil {
			return
		}

		r = append(r, data)
	}

	result = r
	return
}

// FormatLines Formats body as lines of text (delimited by \n)
func FormatLines(body string) (result interface{}, err error) {
	result = strings.Split(body, "\n")
	return
}

// FormatText - Text format just returns body
func FormatText(body string) (result interface{}, err error) {
	result = body
	return
}
