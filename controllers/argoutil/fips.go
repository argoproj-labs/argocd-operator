package argoutil

import (
	"os"
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
