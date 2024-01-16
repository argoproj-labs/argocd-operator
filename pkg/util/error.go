package util

import "errors"

// MultiError employs the generic error interface but allows storage of a slice of errors
type MultiError struct {
	Errs []error
}

// Unwrap returns the slice of collected errors
func (e *MultiError) Unwrap() []error {
	return e.Errs
}

func (e MultiError) Error() string {
	return errors.Join(e.Errs...).Error()
}

// Append adds errors to the slice. Mil errors are filtered out
func (e *MultiError) Append(errs ...error) {
	for _, err := range errs {
		if err == nil {
			continue
		}
		e.Errs = append(e.Errs, err)
	}
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
