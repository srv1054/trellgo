package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/adlio/trello"
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

	version = "0.0.1"

	// Load CLI arguments and OS ENV
	config.ARGS = getCLIArgs()
	config.ENV = getOSENV()

	// Create main directory if doesn't exist

	// Create Trello Client
	client = trello.NewClient(config.ENV.TRELLOAPIKEY, config.ENV.TRELLOAPITOK)

	// Process board data
	board, err := client.GetBoard(config.ARGS.BoardID, trello.Defaults())
	if err != nil {
		fmt.Println("Error: Unable to get board data for board ID", config.ARGS.BoardID)
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Board Name:", board.Name)

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

	config.Archived = *Archived
	config.BoardID = *BoardID
	config.LabelID = *LabelID
	config.ListLabelIDs = *ListLabelIDs
	config.ListTotalCards = *ListTotalCards
	config.StoragePath = *StoragePath

	// Parse CLI flags
	flag.Parse()

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

	// Check for required flag of Storage Path
	if *StoragePath == "" {
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
