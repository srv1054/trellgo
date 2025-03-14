package main

import (
	"fmt"
	"os"

	"github.com/adlio/trello"
)

// dumpABoard - Process the board data and dump to specified directory structure
func dumpABoard(config Config, board *trello.Board, client *trello.Client) {

	var (
		cards []*trello.Card
		err   error
	)

	// Build File System Structure
	// Create main directory if doesn't exist
	dirCreate(config.ARGS.StoragePath)
	// Create directory in path named by board name
	dirCreate(config.ARGS.StoragePath + "/" + board.Name)

	// Get all cards (open unless -a flag is set which includes archived)
	// ### NEED TO HANDLE SPECIFIC LABEL ID REQUESTS
	if config.ARGS.Archived {
		cards, err = board.GetCards(trello.Arguments{"filter": "all"})
	} else {
		cards, err = board.GetCards(trello.Arguments{"filter": "open"})
	}
	if err != nil {
		fmt.Println("Error: Unable to get card data for board ID", config.ARGS.BoardID)
		os.Exit(1)
	}

	// Loop through cards and dump to directory structure
	for _, card := range cards {

		// find cards list name
		list, err := client.GetList(card.IDList, trello.Defaults())
		if err != nil {
			fmt.Println("Error: Unable to get list data for list ID", card.IDList)
			os.Exit(1)
		}
		// create list directory
		dirCreate(config.ARGS.StoragePath + "/" + board.Name + "/" + list.Name)
		// Create directory for card name
		dirCreate(config.ARGS.StoragePath + "/" + board.Name + "/" + list.Name + "/" + card.Name)

		/* Create markdown list of labels and their names/colors
		Save board background image

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
	}
}
