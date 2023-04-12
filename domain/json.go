package domain

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
)

type JsonBytes []byte

func (j *JsonBytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(base64.StdEncoding.EncodeToString(*j))
}

func (j *JsonBytes) UnmarshalJSON(bytes []byte) error {
	var src string
	if err := json.Unmarshal(bytes, &src); err != nil {
		return err
	}

	what, err := base64.StdEncoding.DecodeString(src)
	if err != nil {
		return err
	}

	*j = what

	return nil
}

// JsonStringSlice accepts both string and []string
type JsonStringSlice []string

func (j *JsonStringSlice) UnmarshalJSON(body []byte) error {
	*j = []string{}
	if bytes.HasPrefix(body, []byte("\"")) && bytes.HasSuffix(body, []byte("\"")) {

		var single string

		if err := json.Unmarshal(body, &single); err != nil {
			return err
		}

		*j = []string{single}
		return nil
	}

	var target []string

	if err := json.Unmarshal(body, &target); err != nil {
		return err
	}

	*j = target

	return nil
}
