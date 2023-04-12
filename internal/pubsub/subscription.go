// subscription is a single subscription to a channel or a set of channels.
// simple globbing is supported: * matches any number of characters except of ".".

// subscription understands following channels or groups:
//    - hello
//    - hello.*
//    - hello.*.properties
//    - hello.*$
//    - hello.*.properties$

package pubsub

import (
	"regexp"
	"strings"
)

type subscription struct {
	name   string
	regexp *regexp.Regexp
}

func (s *subscription) Matches(what string) bool {
	return s.regexp.MatchString(what)
}

func (s *subscription) Name() string {
	return s.name
}

func parseSubscription(name string) (_ *subscription, err error) {
	name = strings.Replace(name, "*", "[^\\.]*", -1)

	result := &subscription{}

	if result.regexp, err = regexp.Compile(name); err != nil {
		return nil, err
	}

	return result, nil
}
