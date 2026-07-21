package argoutil

import (
	"os"

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

// GetFIPSGoDebugEnv returns the environment variable GODEBUG set to fips140=on.
func GetFIPSGoDebugEnv() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "GODEBUG",
			Value: "fips140=on",
		},
	}
}

// GetFIPSGoLangEnv returns the environment variable GOLANG_FIPS set to 0. This must be added only if the GODEBUG environment variable contains fips140=on.
func GetFIPSGoLangFipsEnv() []corev1.EnvVar {
	// GOLANG_FIPS and GODEBUG=fips140=on are both mutaully exclusive.
	// GOLANG_FIPS=1 is set by default but it causes issues
	// since we are explicitly setting GODEBUG=fips140=on to skip unsupported fips ssh algorithms in Argo CD.
	// See https://github.com/argoproj/argo-cd/issues/24155,
	// so we need to set GOLANG_FIPS=0 to avoid the conflict.
	return []corev1.EnvVar{
		{
			Name:  "GOLANG_FIPS",
			Value: "0",
		},
	}
}
