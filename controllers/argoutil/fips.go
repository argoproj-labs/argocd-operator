package argoutil

import (
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// FipsConfigChecker defines the behavior for reading the FIPS config.
type FipsConfigChecker interface {
	IsFipsEnabled() (bool, error)
}

// LinuxFipsConfigChecker implements the config reader for checking if the Linux based host is running in FIPS enabled mode.
type LinuxFipsConfigChecker struct {
	ConfigFilePath string
}

// NewLinuxFipsConfigChecker returns a Linux based config checker
func NewLinuxFipsConfigChecker() *LinuxFipsConfigChecker {
	return &LinuxFipsConfigChecker{
		ConfigFilePath: "/proc/sys/crypto/fips_enabled",
	}
}

// IsFipsEnabled reads the specified FIPS config file in a Linux based host and returns true FIPS is enabled, false otherwise.
func (l *LinuxFipsConfigChecker) IsFipsEnabled() (bool, error) {
	found, err := fileExists(l.ConfigFilePath)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	b, err := os.ReadFile(l.ConfigFilePath)
	if err != nil {
		return false, err
	}
	return b[0] == '1', err
}

// fileExists checks if a file with the given path exists
func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}

// DecorateWithFIPSEnv adds environment variables required to enable FIPS in go runtime.
// If user has set this value already, this method does not override it. If the environment variable
// GODEBUG contains fips140=on, it will also set GOLANG_FIPS=0 as these two environments are
// mutually exclusive.
func DecorateWithFIPSEnv(in []corev1.EnvVar) []corev1.EnvVar {
	mergedEnv := EnvMerge(in, []corev1.EnvVar{{
		Name:  "GODEBUG",
		Value: "fips140=on",
	}}, false)
	for _, env := range mergedEnv {
		if env.Name == "GODEBUG" {
			if hasGodebugEntry(env.Value, "fips140=on") {
				// GOLANG_FIPS and GODEBUG=fips140=on are both mutually exclusive.
				// GOLANG_FIPS=1 is set by default, but it causes issues
				// since we are explicitly setting GODEBUG=fips140=on to skip
				// unsupported fips ssh algorithms in Argo CD.
				// See https://github.com/argoproj/argo-cd/issues/24155,
				// so we need to set GOLANG_FIPS=0 to avoid the conflict.
				mergedEnv = EnvMerge(mergedEnv, []corev1.EnvVar{{
					Name:  "GOLANG_FIPS",
					Value: "0",
				}}, false)
			}
			break
		}
	}
	return mergedEnv
}

// hasGodebugEntry checks whether a comma-separated GODEBUG value contains an
// exact entry (e.g. "fips140=on").
func hasGodebugEntry(godebugValue, entry string) bool {
	for _, e := range strings.Split(godebugValue, ",") {
		if e == entry {
			return true
		}
	}
	return false
}
