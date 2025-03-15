package main

import (
	"fmt"
	"os"

	"github.com/adlio/trello"
)

// dumpABoard - Process the board data and dump to specified directory structure
func dumpABoard(config Config, board *trello.Board, client *trello.Client) {

	var (
		cards         []*trello.Card
		err           error
		cleanListPath string
		cleanCardPath string
	)

	// Build File System Structure
	// Create main directory
	dirCreate(config.ARGS.StoragePath)
	// Create directory in path named by board name
	tmpPath := SanitizePathName(board.Name)
	dirCreate(config.ARGS.StoragePath + "/" + tmpPath)

	// Board Level Data
	// Save board background image if exists
	if board.Prefs.BackgroundImage != "" {
		url := board.Prefs.BackgroundImage
		localFilePath := config.ARGS.StoragePath + "/" + board.Name + "/" + "BoardBackground-"
		err := downLoadFile(url, localFilePath)
		if err != nil {
			fmt.Println("Error: Unable to download background image for board", board.Name)
			fmt.Println(err)
		}
	} else {
		fmt.Println("No background image found for board", board.Name)
	}

	//Create markdown list of labels and their names/colors
	fmt.Println("Grabbing labels for board and saving as Markdown BoardLabels.md")
	labels, err := board.GetLabels(trello.Defaults())
	if err != nil {
		fmt.Println("Error: Unable to get label data for board ID "+board.ID+" ("+board.Name+")", config.ARGS.BoardID)
	} else {

		buf := prettyPrintLabels(labels, true)

		// Write buffer content to a file
		labelFileName := config.ARGS.StoragePath + "/" + board.Name + "/" + "BoardLabels.md"
		err := os.WriteFile(labelFileName, buf.Bytes(), 0644)
		if err != nil {
			panic(err)
		}
	}

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
		cleanListPath = SanitizePathName(list.Name)
		dirCreate(config.ARGS.StoragePath + "/" + board.Name + "/" + cleanListPath)
		// Create directory for card name
		cleanCardPath = SanitizePathName(card.Name)
		cardPath := config.ARGS.StoragePath + "/" + board.Name + "/" + cleanListPath + "/" + cleanCardPath
		dirCreate(cardPath)

		// Card Level Data
		fmt.Println("Dumping card:", card.Name)
		// Create markdown file for card description
		err = os.WriteFile(cardPath+"/CardDescription.md", []byte(card.Desc), 0644)
		if err != nil {
			panic(err)
		}
		// Save Attachments - UNTESTED
		// 	check "cover" flag to see if attachment is a cover photo and tag it in the name
		dirCreate(cardPath + "/attachments")
		for _, attachment := range card.Attachments {
			url := attachment.URL
			localFilePath := cardPath + "/attachments/" + attachment.Name
			err := downLoadFile(url, localFilePath)
			fmt.Printf("Downloading attachment: %s\n", attachment.Name)
			if err != nil {
				fmt.Println("Error: Unable to download attachment for card", card.Name)
				fmt.Println(err)
			}

		}

		/*
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
