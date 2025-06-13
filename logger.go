package main

import (
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
