package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Custom error type for parameter errors
type ParameterError struct {
	message string
	cmd     *cobra.Command
}

func (e *ParameterError) Error() string {
	return e.message
}

func NewParameterError(message string) *ParameterError {
	return &ParameterError{message: message, cmd: nil}
}

func NewParameterErrorWithCmd(message string, cmd *cobra.Command) *ParameterError {
	return &ParameterError{message: message, cmd: cmd}
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
			return NewParameterErrorWithCmd(fmt.Sprintf("accepts %d arg(s), received %d", n, len(args)), cmd)
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
