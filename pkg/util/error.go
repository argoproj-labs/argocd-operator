package util

import "errors"

type MultiError struct {
	Errs []error
}

func (e *MultiError) Unwrap() []error {
	return e.Errs
}

func (e MultiError) Error() string {
	return errors.Join(e.Errs...).Error()
}

func (e *MultiError) Append(errs ...error) {
	for _, err := range errs {
		if err == nil {
			continue
		}
		e.Errs = append(e.Errs, err)
	}
}

func (e *MultiError) IsNil() bool {
	return len(e.Errs) == 0
}
