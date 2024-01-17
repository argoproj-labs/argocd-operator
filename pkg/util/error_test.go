package util

import (
	"errors"
	"reflect"
	"testing"
)

func TestMultiError_Split(t *testing.T) {
	tests := []struct {
		name     string
		multiErr *MultiError
		expected []error
	}{
		{
			"Basic split",
			&MultiError{Errs: []error{errors.New("error1"), errors.New("error2")}},
			[]error{errors.New("error1"), errors.New("error2")},
		},
		{
			"split Empty",
			&MultiError{Errs: []error{}},
			[]error{},
		},
		{
			"split MultiError",
			&MultiError{Errs: []error{errors.New("error1"), &MultiError{Errs: []error{errors.New("error2"), errors.New("error3")}}}},
			[]error{errors.New("error1"), &MultiError{Errs: []error{errors.New("error2"), errors.New("error3")}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.multiErr.Split()
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("Unwrap() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMultiError_Error(t *testing.T) {
	tests := []struct {
		name     string
		multiErr *MultiError
		expected string
	}{
		{
			"Basic Error",
			&MultiError{Errs: []error{errors.New("error1"), errors.New("error2")}},
			"error1\nerror2",
		},
		{
			"Error Empty",
			&MultiError{Errs: []error{}},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.multiErr.Error()
			if got != tt.expected {
				t.Errorf("Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMultiError_Append(t *testing.T) {
	tests := []struct {
		name     string
		multiErr *MultiError
		errors   []error
		expected []error
	}{
		{
			"Basic Append",
			&MultiError{Errs: []error{errors.New("error1")}},
			[]error{errors.New("error2")},
			[]error{errors.New("error1"), errors.New("error2")},
		},
		{
			"Append Nil Error",
			&MultiError{Errs: []error{errors.New("error1")}},
			[]error{nil, errors.New("error2")},
			[]error{errors.New("error1"), errors.New("error2")},
		},
		{
			"Append MultiError",
			&MultiError{Errs: []error{errors.New("error1")}},
			[]error{&MultiError{Errs: []error{errors.New("error2"), errors.New("error3")}}},
			[]error{errors.New("error1"), &MultiError{Errs: []error{errors.New("error2"), errors.New("error3")}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.multiErr.Append(tt.errors...)

			got := tt.multiErr.Split()

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("Append() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMultiError_Flatten(t *testing.T) {
	tests := []struct {
		name     string
		multiErr *MultiError
		expected []error
	}{
		{
			"Basic Flatten",
			&MultiError{Errs: []error{errors.New("error1"), errors.New("error2")}},
			[]error{errors.New("error1"), errors.New("error2")},
		},
		{
			"Flatten Nested MultiError",
			&MultiError{Errs: []error{errors.New("error1"), &MultiError{Errs: []error{errors.New("error2"), errors.New("error3")}}}},
			[]error{errors.New("error1"), errors.New("error2"), errors.New("error3")},
		},
		{
			"Flatten Empty MultiError",
			&MultiError{Errs: []error{}},
			[]error{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.multiErr.Flatten()

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("Flatten() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMultiError_IsNil(t *testing.T) {
	tests := []struct {
		name     string
		multiErr *MultiError
		expected bool
	}{
		{
			"IsNil True",
			&MultiError{Errs: []error{}},
			true,
		},
		{
			"IsNil False",
			&MultiError{Errs: []error{errors.New("error1")}},
			false,
		},
		{
			"IsNil MultiError",
			&MultiError{Errs: []error{&MultiError{Errs: []error{errors.New("error2")}}}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.multiErr.IsNil()

			if got != tt.expected {
				t.Errorf("IsNil() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMultiError_ErrOrNil(t *testing.T) {

	testME := MultiError{Errs: []error{errors.New("error1")}}
	testMultiME := MultiError{Errs: []error{&MultiError{Errs: []error{errors.New("error2")}}}}

	tests := []struct {
		name     string
		multiErr *MultiError
		expected error
	}{
		{
			"ErrOrNil Nil",
			&MultiError{Errs: []error{}},
			nil,
		},
		{
			"ErrOrNil Non-Nil",
			&testME,
			&testME,
		},
		{
			"ErrOrNil MultiError",
			&testMultiME,
			&testMultiME,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.multiErr.ErrOrNil()

			if got != tt.expected {
				t.Errorf("ErrOrNil() = %v, want %v", got, tt.expected)
			}
		})
	}
}
