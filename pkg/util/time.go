package util

import (
	"fmt"
	"time"
)

// nowNano returns a string with the current UTC time as epoch in nanoseconds
func NowNano() string {
	return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
}
