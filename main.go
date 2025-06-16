package main

import (
	"fmt"
	"os"

	"github.com/adlio/trello"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// GLobal
var (
	version      string
	listOfBoards []string
	boardTracker []string // Used to track boards that have been processed to reference at the end of the run
	ListLoud     bool
	config       Config
	client       *trello.Client
)

type Config struct {
	ARGS ARGS
	ENV  ENV
}

func main() {

	// Major.Feature.Patch
	version = "0.3.09"

	// Load CLI arguments and OS ENV
	// This also must handle stdin Pipe input
	config.ARGS, listOfBoards = getCLIArgs()
	config.ENV = getOSENV()

	// Create Log File if Enabled
	if config.ARGS.LogFile != "" {
		if startLog(config) {
			config.ARGS.LoggingEnabled = true
			logger("Successfully started log file: "+config.ARGS.LogFile, "info", true, false, config)
		} else {
			config.ARGS.LoggingEnabled = false
		}
	}

	// Announce we are starting, this will only go to logfile if its enabled
	logger("Starting New Trellgo (v"+version+") run.", "info", false, false, config)

	// Create Trello Client
	client = trello.NewClient(config.ENV.TRELLOAPIKEY, config.ENV.TRELLOAPITOK)

	// Message this once outside the loop, rather than for each board on multiple board input
	if config.ARGS.ListTotalCards {
		logger("\n\nLarge Boards will take a moment to retreive this data...\n\n", "info", true, false, config)
	}

	// Range through board IDs.  Came in via CLI args or stdin pipe
	for _, boardID := range listOfBoards {

		// validate board ID by getting the board data
		board, err := client.GetBoard(boardID, trello.Defaults())
		if err != nil {
			logger("Error: Unable to get board data for board ID"+boardID+": "+err.Error(), "err", true, false, config)
			continue
		}

		/* Process Label List Request (-labels) */
		if config.ARGS.ListLabelIDs {

			labels, err := board.GetLabels(trello.Defaults())
			if err != nil {
				logger("Error: Unable to get label data for board ID "+board.ID+" ("+board.Name+"): "+err.Error(), "err", true, false, config)
				continue
			}

			fmt.Printf("\n\nLabel IDs for Board: %s (%s)\n\n", board.Name, board.ID)
			prettyPrintLabels(labels, false)

			continue
		}

		/* Process Card Counts Request (-count) */
		if config.ARGS.ListTotalCards {

			totalCards, _ := board.GetCards(trello.Arguments{"filter": "all"})
			openCards, _ := board.GetCards(trello.Arguments{"filter": "open"})
			closedCards, _ := board.GetCards(trello.Arguments{"filter": "closed"})
			visibleCards, _ := board.GetCards(trello.Arguments{"filter": "visible"}) // Visible cards are open and not archived

			t := table.NewWriter()
			t.SetOutputMirror(os.Stdout)
			t.AppendRow([]interface{}{"Total Cards", len(totalCards)})
			t.AppendSeparator()
			t.AppendRow([]interface{}{"Open Cards", len(openCards)})
			t.AppendSeparator()
			t.AppendRow([]interface{}{"Archived Cards", len(closedCards)})
			t.AppendSeparator()
			t.AppendRow([]interface{}{"Visible Cards", len(visibleCards)})

			t.SetStyle(table.StyleLight)
			t.Style().Color.Header = text.Colors{text.FgHiGreen, text.Bold}

			fmt.Printf("\n\nCard Counts for Board: %s (%s)\n\n", board.Name, board.ID)

			t.Render()

			fmt.Println()

			continue
		}

		/* Process board data (-b) or (stdin pipe) */
		if !config.ARGS.ListLabelIDs && !config.ARGS.ListTotalCards {
			if !config.ARGS.SuperQuiet {
				fmt.Println()
			}
			logger("Processing Board Name: "+board.Name, "info", true, false, config)
			dumpABoard(config, board, client)

			if !config.ARGS.SuperQuiet {
				fmt.Println()
			}
			logger("Processing Complete", "info", true, false, config)
		}
	}

	if !config.ARGS.ListLabelIDs && !config.ARGS.ListTotalCards {
		logger("Your board backups are in the directory:"+config.ARGS.StoragePath, "info", true, false, config)
	}

	// If we have processed boards, print them out
	if len(boardTracker) > 0 {
		fmt.Println("\nBoards Processed:")
		for _, boardName := range boardTracker {
			logger(" - "+boardName, "info", true, false, config)
		}
	}
}
