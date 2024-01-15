package util

import "errors"

type MultiError struct {
	Errs []error
}

func (e *MultiError) Unwrap() []error {
	return e.Errs
}

func (e *MultiError) Error() string {
	return errors.Join(e.Errs...).Error()
}
