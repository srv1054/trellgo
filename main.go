package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/adlio/trello"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// What are we doing
/*
	Provide triggers on which cards to grab
		board ID
		All cards
			default no : include archived yes/no
			default none : with a label ID
		Only archived cards
		List labels to get IDs
		List total number of cards (and archived cards)
	Need to read Trello API out of OS ENV Var
	Need to provide storage directory path
	    Create markdown list of labels and their names/colors
		Save board background image
		Create directory in path named by board name
			Create directory in board directory named by list name
				Create directory in list directory named by card name
		--------------------------------------
		In Card Directory store:
			Card Description markdown
			Card Checklist markdown (including properly checked items)
			Directory called attachments that stores:
				File attachments (tag cover photo in name)
				Links
			Card Comments markdown
			Card Labels text file
			Card History in text file
*/
// Steps
/*
	Build CLI manager
	Read OSENV for Trello API key and token
	Create main directory if doesn't exist
*/

// GLobal
var (
	version string
	config  Config
	client  *trello.Client
)

type ARGS struct {
	BoardID        string
	Archived       bool
	LabelID        string
	ListLabelIDs   bool
	ListTotalCards bool
	StoragePath    string
}
type ENV struct {
	TRELLOAPIKEY string
	TRELLOAPITOK string
	TRELLOAPIURL string
}
type Config struct {
	ARGS ARGS
	ENV  ENV
}

func main() {

	version = "0.0.8"

	// Load CLI arguments and OS ENV
	config.ARGS = getCLIArgs()
	config.ENV = getOSENV()

	if config.ARGS.ListLabelIDs {

	}

	// Create Trello Client
	client = trello.NewClient(config.ENV.TRELLOAPIKEY, config.ENV.TRELLOAPITOK)

	// Process Label List Request
	if config.ARGS.ListLabelIDs {

		board, err := client.GetBoard(config.ARGS.BoardID, trello.Defaults())
		if err != nil {
			fmt.Println("Error: Unable to get board data for board ID", config.ARGS.BoardID)
			fmt.Println(err)
			os.Exit(1)
		}

		labels, err := board.GetLabels(trello.Defaults())
		if err != nil {
			fmt.Println("Error: Unable to get label data for board ID "+board.ID+" ("+board.Name+")", config.ARGS.BoardID)
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Println()
		fmt.Println("Label IDs for Board ID:", board.ID, "Board Name:", board.Name)
		fmt.Println()
		prettyPrintLabels(labels, false)
		fmt.Println()
		os.Exit(0)
	}

	// Process Card Counts Request
	if config.ARGS.ListTotalCards {
		os.Exit(0)
	}

	// Process board data

	// Create main directory if doesn't exist
	dirCreate(config.ARGS.StoragePath)

	board, err := client.GetBoard(config.ARGS.BoardID, trello.Defaults())
	if err != nil {
		fmt.Println("Error: Unable to get board data for board ID", config.ARGS.BoardID)
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Processing Board Name:", board.Name)

}

// getCLIArgs - Get CLI arguments and flags
func getCLIArgs() (config ARGS) {

	var (
		// CLI Flags
		ver            = flag.Bool("v", false, "Version Check")
		BoardID        = flag.String("b", "", "Trello board to dump Unique Identifier")
		Archived       = flag.Bool("a", false, "Include archived cards in dump")
		LabelID        = flag.String("l", "", "Only include cards with this label ID")
		ListLabelIDs   = flag.Bool("labels", false, "Retrieve boards list of Label IDs")
		ListTotalCards = flag.Bool("count", false, "List total number of cards in the board")
		StoragePath    = flag.String("s", "", "Root Level path to store board information")
	)

	// Handle -h help
	flag.Usage = func() { printHelp(version) }

	// Parse CLI flags
	flag.Parse()

	// Set config values
	config.Archived = *Archived
	config.BoardID = *BoardID
	config.LabelID = *LabelID
	config.ListLabelIDs = *ListLabelIDs
	config.ListTotalCards = *ListTotalCards
	config.StoragePath = *StoragePath

	// Handle -v version
	if *ver {
		fmt.Printf("trellgo v%s\n", version)
		os.Exit(0)
	}

	// Check for required flag of Board ID
	if *BoardID == "" {
		fmt.Println("Error: No Board ID provided. REQUIRED")
		printHelp(version)
		os.Exit(1)
	}

	// Check for required flag of Storage Path if not listing labels or card totals
	if !*ListLabelIDs && !*ListTotalCards && *StoragePath == "" {
		fmt.Println("Error: No Storage Path provided. REQUIRED")
		printHelp(version)
		os.Exit(1)
	}

	return config
}

// getOSENV - Get Trello API Key from OS Environment
func getOSENV() (config ENV) {

	config.TRELLOAPIKEY = os.Getenv("TRELLGO_APIKEY")
	config.TRELLOAPITOK = os.Getenv("TRELLGO_APITOK")
	config.TRELLOAPIURL = os.Getenv("TRELLGO_APIURL")

	return config
}

// printHelp - prints help menu when -h is used on CLI
func printHelp(version string) {
	fmt.Printf("\ttrellgo v%s by srv1054 (github.com/srv1054/trellgo)\n", version)
	fmt.Println()
	fmt.Println("Usage: ./trellgo [options]")
	fmt.Println("Options:")
	fmt.Printf("  -a\t\tInclude archived cards in dump (REQUIRED)\n")
	fmt.Printf("  -b\t\tTrello board to dump BoardID\n")
	fmt.Printf("  -l\t\tOnly include cards with this label ID\n")
	fmt.Printf("  -labels\tRetrieve boards list of Label IDs\n")
	fmt.Printf("  -count\tList total number of cards in the board\n")
	fmt.Printf("  -s\t\tRoot Level path to store board information (REQUIRED)\n")
	fmt.Printf("  -v\t\ttPrints version and exits\n")
	fmt.Println()
	fmt.Printf("Example: trellgo -b c52d11s -l ff3sg135 -s '/path/to/here'\n")
	fmt.Printf("Example: trellgo -labels")
	fmt.Println()
	os.Exit(0)
}

func dirCreate(storagePath string) {
	// Check if main directory exists, if not create it
	if _, err := os.Stat(storagePath); os.IsNotExist(err) {
		fmt.Println("Creating main directory:", storagePath)
		err := os.MkdirAll(storagePath, os.ModePerm)
		if err != nil {
			fmt.Println("Error: Unable to create main directory:", storagePath)
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("Main directory created:", storagePath)
	} else {
		fmt.Println("Main directory already exists:", storagePath)
	}
}

func prettyPrintLabels(labels []*trello.Label, markdown bool) {

	var (
		t = table.NewWriter()
	)

	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Label Name", "Label Color", "Label UID"})

	for _, label := range labels {
		t.AppendRow([]interface{}{label.Name, label.Color, label.ID})
		t.AppendSeparator()
	}

	// Set style
	t.SetStyle(table.StyleLight)
	t.Style().Color.Header = text.Colors{text.BgHiYellow, text.FgBlack}
	t.Style().Color.Footer = text.Colors{text.BgHiYellow, text.FgBlack}

	// Render a markdown table
	if markdown {
		fmt.Println()
		t.RenderMarkdown()
	} else {
		// Render normal table to console
		t.Render()
	}

	fmt.Println()
}
