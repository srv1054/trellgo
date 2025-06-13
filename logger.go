package main

import "fmt"

/*
logger
	Deal with outputs, console, log files, -qq, -loud, etc
	console - true to send to console
	honorLoud - honor the global variable ListLoud (-loud cli parameter)
	config - Send in our config struct
*/
func logger(message string, console bool, honorLoud bool, config Config) {

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

	// Spot here for file logging when we implement that
	// If logging is enabled, send everything to logs regardless of parameters

}
