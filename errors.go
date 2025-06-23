package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Custom error type for parameter errors
type ParameterError struct {
	message string
}

func (e *ParameterError) Error() string {
	return e.message
}

func NewParameterError(message string) *ParameterError {
	return &ParameterError{message: message}
}

// Check if error is a parameter-related error
func isParameterError(err error) bool {
	_, ok := err.(*ParameterError)
	return ok
}

// Custom argument validator that returns ParameterError
func exactArgsWithParameterError(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return NewParameterError(fmt.Sprintf("accepts %d arg(s), received %d", n, len(args)))
		}
		return nil
	}
}

// Format error message with red "Error:" prefix
func formatError(err error) string {
	errMsg := err.Error()

	if len(errMsg) >= 6 && errMsg[:6] == "Error:" {
		return colorRed + "Error:" + colorReset + errMsg[6:]
	}

	return colorRed + "Error:" + colorReset + " " + errMsg
}
