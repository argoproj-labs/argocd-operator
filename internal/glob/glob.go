// Package glob provides glob and regex pattern matching utilities.
// Copied from github.com/argoproj/argo-cd/v3/util/glob to remove the dependency.
package glob

import (
	"strings"

	"github.com/dlclark/regexp2"
	"github.com/gobwas/glob"
)

const (
	EXACT  = "exact"
	GLOB   = "glob"
	REGEXP = "regexp"
)

// Match tries to match a text with a given glob pattern.
func Match(pattern, text string, separators ...rune) bool {
	compiledGlob, err := glob.Compile(pattern, separators...)
	if err != nil {
		return false
	}
	return compiledGlob.Match(text)
}

// regexMatch checks whether text matches a regexp2 pattern.
func regexMatch(pattern, text string) bool {
	compiledRegex, err := regexp2.Compile(pattern, 0)
	if err != nil {
		return false
	}
	regexMatch, err := compiledRegex.MatchString(text)
	if err != nil {
		return false
	}
	return regexMatch
}

// MatchStringInList returns true if item matches any entry in list.
// patternMatch can be set to EXACT, GLOB, or REGEXP.
// If REGEXP, strings wrapped in "/" are treated as regular expressions.
func MatchStringInList(list []string, item string, patternMatch string) bool {
	for _, ll := range list {
		switch {
		case patternMatch == REGEXP && strings.HasPrefix(ll, "/") && strings.HasSuffix(ll, "/") && regexMatch(ll[1:len(ll)-1], item):
			return true
		case (patternMatch == REGEXP || patternMatch == GLOB) && Match(ll, item):
			return true
		case patternMatch == EXACT && item == ll:
			return true
		}
	}
	return false
}
