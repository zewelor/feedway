package entry

import (
	"errors"
	"regexp"
)

var feedPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type Feed struct {
	value string
}

func ParseFeed(value string) (Feed, error) {
	if len(value) < 1 || len(value) > 64 || !feedPattern.MatchString(value) {
		return Feed{}, errors.New("feed must be 1 to 64 lowercase ASCII letters or digits separated by single hyphens")
	}
	return Feed{value: value}, nil
}

func (f Feed) String() string {
	return f.value
}
