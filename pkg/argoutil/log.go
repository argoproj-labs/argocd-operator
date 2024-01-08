package argoutil

import (
	"strings"

	"github.com/argoproj-labs/argocd-operator/common"
)

// GetLogLevel returns the log level for a specified component if it is set or returns the default log level if it is not set
func GetLogLevel(logField string) string {

	switch strings.ToLower(logField) {
	case "debug",
		"info",
		"warn",
		"error":
		return logField
	}
	return common.ArgoCDDefaultLogLevel
}

// GetLogFormat returns the log format for a specified component if it is set or returns the default log format if it is not set
func GetLogFormat(logField string) string {
	switch strings.ToLower(logField) {
	case "text",
		"json":
		return logField
	}
	return common.ArgoCDDefaultLogFormat
}
