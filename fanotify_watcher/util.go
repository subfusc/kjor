package fanotify_watcher

import "regexp"

func regexpAny(matchers []*regexp.Regexp, against string) bool {
	for _, matcher := range matchers {
		if matcher.MatchString(against) {
			return true
		}
	}

	return false
}
