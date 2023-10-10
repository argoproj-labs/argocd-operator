package util

import (
	"fmt"
	"time"
)

// nowBytes is a shortcut function to return the current date/time in RFC3339 format.
func NowBytes() []byte {
	return []byte(time.Now().UTC().Format(time.RFC3339))
}

// nowNano returns a string with the current UTC time as epoch in nanoseconds
func NowNano() string {
	return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
}