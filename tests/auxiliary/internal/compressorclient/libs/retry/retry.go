package retry

import (
	"fmt"
	"smtplistener/internal/compressorclient/pb"
	object "smtplistener/internal/processorobject"
	"strings"
	"time"
)

// Function signature of retryable function
type RetryableFunc func() error

type RessilientRetryableFunc func() (pb.MTACompressResponse, error)

var (
	DefaultAttempts      uint
	DefaultDelay         time.Duration
	DefaultMaxJitter     time.Duration
	DefaultOnRetry       = func(n uint, err error) {}
	DefaultRetryIf       = IsRecoverable
	DefaultDelayType     = CombineDelay(BackOffDelay, RandomDelay)
	DefaultLastErrorOnly bool
)

func SetConfig(retryConfig object.RetryConfig) {
	DefaultAttempts = retryConfig.DefaultAttempts
	DefaultDelay = retryConfig.DefaultDelay * time.Second
	DefaultMaxJitter = retryConfig.DefaultMaxJitter * time.Second
	DefaultLastErrorOnly = retryConfig.DefaultLastErrorOnly
}

func Do(retryableFunc RetryableFunc, opts ...Option) error {
	var n uint

	//default
	config := &Config{
		attempts:      DefaultAttempts,
		delay:         DefaultDelay,
		maxJitter:     DefaultMaxJitter,
		onRetry:       DefaultOnRetry,
		retryIf:       DefaultRetryIf,
		delayType:     DefaultDelayType,
		lastErrorOnly: DefaultLastErrorOnly,
	}

	//apply opts
	for _, opt := range opts {
		opt(config)
	}

	var errorLog Error
	if !config.lastErrorOnly {
		errorLog = make(Error, config.attempts)
	} else {
		errorLog = make(Error, 1)
	}

	lastErrIndex := n
	for n < config.attempts {
		err := retryableFunc()

		if err != nil {
			errorLog[lastErrIndex] = unpackUnrecoverable(err)

			if !config.retryIf(err) {
				break
			}

			config.onRetry(n, err)

			// if this is last attempt - don't wait
			if n == config.attempts-1 {
				break
			}

			delayTime := config.delayType(n, config)
			if config.maxDelay > 0 && delayTime > config.maxDelay {
				delayTime = config.maxDelay
			}
			time.Sleep(delayTime)
		} else {
			return nil
		}

		n++
		if !config.lastErrorOnly {
			lastErrIndex = n
		}
	}

	if config.lastErrorOnly {
		return errorLog[lastErrIndex]
	}
	return errorLog
}

// Error type represents list of errors in retry
type Error []error

// Error method return string representation of Error
// It is an implementation of error interface
func (e Error) Error() string {
	logWithNumber := make([]string, lenWithoutNil(e))
	for i, l := range e {
		if l != nil {
			logWithNumber[i] = fmt.Sprintf("#%d: %s", i+1, l.Error())
		}
	}

	return fmt.Sprintf("All attempts fail:\n%s", strings.Join(logWithNumber, "\n"))
}

func lenWithoutNil(e Error) (count int) {
	for _, v := range e {
		if v != nil {
			count++
		}
	}

	return
}

func (e Error) WrappedErrors() []error {
	return e
}

type unrecoverableError struct {
	error
}

// Unrecoverable wraps an error in `unrecoverableError` struct
func Unrecoverable(err error) error {
	return unrecoverableError{err}
}

// IsRecoverable checks if error is an instance of `unrecoverableError`
func IsRecoverable(err error) bool {
	_, isUnrecoverable := err.(unrecoverableError)
	return !isUnrecoverable
}

func unpackUnrecoverable(err error) error {
	if unrecoverable, isUnrecoverable := err.(unrecoverableError); isUnrecoverable {
		return unrecoverable.error
	}

	return err
}

func RessilientDo(retryableFunc RessilientRetryableFunc, opts ...Option) (pb.MTACompressResponse, error) {
	var n uint

	//default
	config := &Config{
		attempts:      DefaultAttempts,
		delay:         DefaultDelay,
		maxJitter:     DefaultMaxJitter,
		onRetry:       DefaultOnRetry,
		retryIf:       DefaultRetryIf,
		delayType:     DefaultDelayType,
		lastErrorOnly: DefaultLastErrorOnly,
	}

	//apply opts
	for _, opt := range opts {
		opt(config)
	}

	var errorLog Error
	if !config.lastErrorOnly {
		errorLog = make(Error, config.attempts)
	} else {
		errorLog = make(Error, 1)
	}

	lastErrIndex := n
	for n < config.attempts {
		resp, err := retryableFunc()

		if err != nil || resp.MtaSuccessMessage == false {
			errorLog[lastErrIndex] = unpackUnrecoverable(err)

			if !config.retryIf(err) {
				break
			}

			config.onRetry(n, err)

			// if this is last attempt - don't wait
			if n == config.attempts-1 {
				break
			}

			delayTime := config.delayType(n, config)
			if config.maxDelay > 0 && delayTime > config.maxDelay {
				delayTime = config.maxDelay
			}
			time.Sleep(delayTime)
		} else {
			return resp, nil
		}

		n++
		if !config.lastErrorOnly {
			lastErrIndex = n
		}
	}
	routerResp := pb.MTACompressResponse{MtaSuccessMessage: false}
	if config.lastErrorOnly {
		return routerResp, errorLog[lastErrIndex]
	}
	return routerResp, errorLog
}
