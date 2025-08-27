package main

import (
	"errors"
	"fmt"
	"log"
	"os"
)

var (
	WarningLogger *log.Logger
	InfoLogger    *log.Logger
	ErrorLogger   *log.Logger
)

/*
logger

	Deal with outputs, console, log files, -qq, -loud, etc
	console - true to send to console
	honorLoud - honor the global variable ListLoud (-loud cli parameter)
	config - Send in our config struct
*/
func logger(message string, state string, console bool, honorLoud bool, config Config) {

	// should we send to console
	if console {
		// over-ride if -qq super quiet mode is set
		if !config.ARGS.SuperQuiet {
			if honorLoud {
				if ListLoud {
					fmt.Println(message)
				}
			} else {
				fmt.Println(message)
			}
		}
	}

	// If logging is enabled, send everything to logs regardless of CLI parameters
	if config.ARGS.LoggingEnabled {
		// Looks like we are logging
		switch state {
		case "warn":
			WarningLogger.Println(message)
		case "info":
			InfoLogger.Println(message)
		case "err":
			ErrorLogger.Println(message)
		}
	}
}

/*
startLog - create log file if enabled
returns true or false depending on successful log file creation
*/
func startLog(config Config) bool {

	if config.ARGS.LogFile == "" {
		return false
	}

	filename := config.ARGS.LogFile

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		fmt.Println("Failed to initiate log file, specified in -logs called: " + filename)
		fmt.Println(err)

		return false
	}

	InfoLogger = log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	WarningLogger = log.New(file, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(file, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	return true
}

/*
Error handling types and utilities for consistent error management
*/

// ErrorSeverity defines the severity level of errors
type ErrorSeverity int

const (
	ErrorSeverityWarning  ErrorSeverity = iota // Non-critical, continue processing
	ErrorSeverityError                         // Significant error, skip current item
	ErrorSeverityCritical                      // Fatal error, stop processing
)

// ProcessingError wraps errors with context and severity
type ProcessingError struct {
	Operation string
	Context   string
	Severity  ErrorSeverity
	Err       error
}

func (e *ProcessingError) Error() string {
	return fmt.Sprintf("%s failed for %s: %v", e.Operation, e.Context, e.Err)
}

func (e *ProcessingError) Unwrap() error {
	return e.Err
}

// newProcessingError creates a new processing error with context
func newProcessingError(operation, context string, severity ErrorSeverity, err error) *ProcessingError {
	return &ProcessingError{
		Operation: operation,
		Context:   context,
		Severity:  severity,
		Err:       err,
	}
}

// handleProcessingError handles errors consistently based on severity
func handleProcessingError(err error, config Config) error {
	if err == nil {
		return nil
	}

	var procErr *ProcessingError
	if !errors.As(err, &procErr) {
		// Wrap non-ProcessingError errors as warnings
		procErr = newProcessingError("unknown operation", "unknown context", ErrorSeverityWarning, err)
	}

	// Log based on severity
	switch procErr.Severity {
	case ErrorSeverityWarning:
		logger("Warning: "+procErr.Error(), "warn", true, true, config)
		return nil // Continue processing
	case ErrorSeverityError:
		logger("Error: "+procErr.Error(), "err", true, false, config)
		return procErr // Skip current item but continue
	case ErrorSeverityCritical:
		logger("CRITICAL: "+procErr.Error(), "err", true, false, config)
		errorWarnOnCompletion = true
		return procErr // Stop processing
	default:
		logger("Unknown error severity: "+procErr.Error(), "err", true, false, config)
		return procErr
	}
}
