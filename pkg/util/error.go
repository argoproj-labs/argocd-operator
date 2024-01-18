package util

import (
	"errors"
)

// MultiError employs the generic error interface but allows storage of a slice of errors
type MultiError struct {
	Errs []error
}

// Split returns the slice of collected errors
func (e *MultiError) Split() []error {
	return e.Errs
}

func (e MultiError) Error() string {
	if e.IsNil() {
		return ""
	}
	return errors.Join(e.Errs...).Error()
}

// Append adds errors to the slice. Nil errors are filtered out.
// Errors are not flattened out
func (e *MultiError) Append(errs ...error) {
	for _, err := range errs {
		if err == nil {
			continue
		}
		e.Errs = append(e.Errs, err)
	}
}

// Flatten recursively flattens out a MultiError
func (e *MultiError) Flatten() []error {
	var flattenedErrors []error = make([]error, 0)

	for _, err := range e.Errs {
		if subMultiErr, ok := err.(*MultiError); ok {
			// If the error is a MultiError, recursively flatten it
			flattenedErrors = append(flattenedErrors, subMultiErr.Flatten()...)
		} else {
			// Otherwise, add the error to the flattened list
			flattenedErrors = append(flattenedErrors, err)
		}
	}

	return flattenedErrors
}

// IsNil determins if the MultiError by checking if the length of the error slice is 0 or not
func (e *MultiError) IsNil() bool {
	return len(e.Errs) == 0
}

// ErrOrNil returns nil if the given error is determined to be nil, else it returns the error itself
func (e *MultiError) ErrOrNil() error {
	if e.IsNil() {
		return nil
	}
	return e
}
