// Copyright 2019 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

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

// getLogFormat returns the log format for a specified component if it is set or returns the default log format if it is not set
func GetLogFormat(logField string) string {
	switch strings.ToLower(logField) {
	case "text",
		"json":
		return logField
	}
	return common.ArgoCDDefaultLogFormat
}
