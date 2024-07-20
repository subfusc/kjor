package common

import "regexp"

func RegexpAny(matchers []*regexp.Regexp, against string) bool {
	for _, matcher := range matchers {
		if matcher.MatchString(against) {
			return true
		}
	}

	return false
}
