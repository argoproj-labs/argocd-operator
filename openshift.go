//go:build openshift
// +build openshift

package main

import (
	_ "github.com/argoproj-labs/argocd-operator/controllers/openshift"
)
