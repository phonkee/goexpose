package domain

import "net/http"

/*
Authorizer implements authorization
*/
type Authorizer interface {
	Authorize(r *http.Request) error
}

type Authorizers map[string]Authorizer

// Authorize Try all authorizers, first that will fail with error, that error will be returned
func (a Authorizers) Authorize(r *http.Request, config *EndpointConfig) (err error) {
	var check []string
	for _, an := range config.Authorizers {
		check = append(check, an)
	}
	for _, an := range config.Methods[r.Method].Authorizers {
		check = append(check, an)
	}

	for _, an := range check {
		authorizer := a[an]
		if err = authorizer.Authorize(r); err != nil {
			return
		}
	}
	return
}

// Names Returns names of all authorizerse
func (a Authorizers) Names() []string {
	result := make([]string, 0, len(a))
	for k, _ := range a {
		result = append(result, k)
	}
	return result
}