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
	version string
	config  Config
	client  *trello.Client
)

type Config struct {
	ARGS ARGS
	ENV  ENV
}

func main() {

	version = "0.1.01"

	// Load CLI arguments and OS ENV
	config.ARGS = getCLIArgs()
	config.ENV = getOSENV()

	// Create Trello Client
	client = trello.NewClient(config.ENV.TRELLOAPIKEY, config.ENV.TRELLOAPITOK)

	// Process Label List Request
	if config.ARGS.ListLabelIDs {

		board, err := client.GetBoard(config.ARGS.BoardID, trello.Defaults())
		if err != nil {
			fmt.Println("Error: Unable to get board data for board ID", config.ARGS.BoardID)
			os.Exit(1)
		}

		labels, err := board.GetLabels(trello.Defaults())
		if err != nil {
			fmt.Println("Error: Unable to get label data for board ID "+board.ID+" ("+board.Name+")", config.ARGS.BoardID)
			os.Exit(1)
		}

		fmt.Printf("\n\nLabel IDs for Board: %s (%s)\n\n", board.Name, board.ID)
		prettyPrintLabels(labels, false)

		os.Exit(0)
	}

	// Process Card Counts Request
	if config.ARGS.ListTotalCards {

		board, err := client.GetBoard(config.ARGS.BoardID, trello.Defaults())
		if err != nil {
			fmt.Println("Error: Unable to get board data for board ID", config.ARGS.BoardID)
			os.Exit(1)
		}

		fmt.Printf("\n\nLarge Boards will take a moment to retreive this data...\n\n")
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
		t.Render()

		fmt.Println()
		os.Exit(0)
	}

	// Process board data
	board, err := client.GetBoard(config.ARGS.BoardID, trello.Defaults())
	if err != nil {
		fmt.Println("Error: Unable to get board data for board ID", config.ARGS.BoardID)
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Processing Board Name:", board.Name)
	dumpABoard(config, board, client)

	fmt.Println("Processing Complete")

}
