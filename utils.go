package goexpose

import (
	"bytes"
	"strings"
	"text/template"
)

/*
Returns if method is allowed

if avail methods is blank it also returns true
*/
func MethodAllowed(method string, avail []string) bool {
	if len(avail) == 0 {
		return true
	}
	for _, am := range avail {
		if strings.ToUpper(method) == strings.ToUpper(am) {
			return true
		}
	}
	return false
}

/*
Interpolate

renders template with data
*/
func Interpolate(strTemplate string, data map[string]interface{}) (result string, err error) {

	var tpl *template.Template

	// compile url to template
	if tpl, err = template.New("anonym-template").Parse(strTemplate); err != nil {
		return

	}

	return RenderTemplate(tpl, data)
}

/*
RenderTemplate

renders template with data
*/
func RenderTemplate(tpl *template.Template, data map[string]interface{}) (result string, err error) {

	b := bytes.NewBuffer([]byte{})

	// interpolate url
	if err = tpl.Execute(b, data); err != nil {
		return
	}

	return b.String(), nil
}
