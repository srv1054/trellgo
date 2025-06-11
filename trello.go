package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/adlio/trello"
)

// dumpABoard - Process the board data and dump to specified directory structure
func dumpABoard(config Config, board *trello.Board, client *trello.Client) {

	var (
		cards         []*trello.Card
		err           error
		cleanListPath string
		cleanCardPath string
		cardPath      string
		cardNumber    int
		buff          bytes.Buffer
	)

	/*
		Build File System Structure
	*/
	// Create main directory
	dirCreate(config.ARGS.StoragePath)
	// Create directory in path named by board name
	tmpPath := SanitizePathName(board.Name)
	dirCreate(config.ARGS.StoragePath + "/" + tmpPath)

	/*
		Board Level Data
	*/
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

	/*
		Create markdown list of labels and their names/colors
	*/
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

	/*
		Get all cards (open unless -a flag is set which includes archived)
	*/
	// ### STILL NEED TO HANDLE SPECIFIC LABEL ID REQUESTS FEATURE
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
		// If card is archived, append -ARCHIVED to the card name or move to ARCHIVED directory
		if card.Closed {
			if !config.ARGS.SeparateArchived {
				// If -split flag is not set, append -ARCHIVED to the card name
				cardPath = config.ARGS.StoragePath + "/" + board.Name + "/" + cleanListPath + "/" + cleanCardPath + " (ARCHIVED)"
				// If -split flag is set, move to ARCHIVED directory
			} else {
				cardPath = config.ARGS.StoragePath + "/" + board.Name + "/ARCHIVED/" + cleanListPath + "/" + cleanCardPath
			}
			// card is not archived
		} else {
			cardPath = config.ARGS.StoragePath + "/" + board.Name + "/" + cleanListPath + "/" + cleanCardPath
		}

		dirCreate(cardPath)

		/*
			Card Level Data
		*/
		fmt.Println("Dumping card:", card.Name)
		// Create markdown file for card description
		err = os.WriteFile(cardPath+"/CardDescription.md", []byte(card.Desc), 0644)
		if err != nil {
			panic(err)
		}

		/*
			Save Card Attachments
			- Download file attachments
			- Save URL attachments in a text markdown file
		*/
		// 	check "cover" flag to see if attachment is a cover photo and tag it in the name
		attachments, err := card.GetAttachments(trello.Defaults())
		if err != nil {
			fmt.Println("Error: Unable to get attachment data for card ID", card.ID)
			fmt.Println(err)
		}

		// Clear the old Bytes Buffer
		buff.Reset()

		if len(attachments) > 0 {
			dirCreate(cardPath + "/attachments")
			fmt.Println(card.Name + " has  " + strconv.Itoa(len(attachments)) + " attachments")

			for _, a := range attachments {

				if a.IsUpload {
					// Download
					filePath := cardPath + "/attachments/"
					// Format https://api.trello.com/1/cards/{idCard}/attachments/{idAttachment}/download/{attachmentFileName}
					authURL := fmt.Sprintf("https://api.trello.com/1/cards/%s/attachments/%s/download/%s", card.ID, a.ID, a.Name)
					err := downloadFileAuthHeader(authURL, filePath, config.ENV.TRELLOAPIKEY, config.ENV.TRELLOAPITOK)
					if err != nil {
						fmt.Println("Error downloading attachment from " + authURL + " to " + filePath)
						fmt.Println(err)
					}
				} else {
					// build a bytes.buffer
					buff.WriteString(a.URL)
					buff.WriteString("\n")
				}

			}

			// Write buffer to disc for URL Attachments
			err := os.WriteFile(cardPath+"/attachments/URL-Attachments.md", buff.Bytes(), 0644)
			if err != nil {
				panic(err)
			}
		} else {
			fmt.Println("No attachments found for card", card.Name)
		}

		/*
			Save Card Checklists
			- Create markdown file for each checklist
			- Include checked items in markdown
		*/
		cardNumber = 0
		fmt.Println("Found", len(card.IDCheckLists), "checklists for card", card.Name)

		for _, checkList := range card.IDCheckLists {
			// Get checklist data
			args := trello.Arguments{"checkItems": "all"}
			checklist, err := client.GetChecklist(checkList, args)

			checklistName := SanitizePathName(checklist.Name)
			for _, item := range checklist.CheckItems {
				// If item is checked, append [x] to the name, otherwise append [ ]
				if item.State == "complete" {
					buff.WriteString(fmt.Sprintf("- [x] %s\n", item.Name))
				} else {
					buff.WriteString(fmt.Sprintf("- [ ] %s\n", item.Name))
				}
			}

			fullpath := filepath.Join(cardPath, checklistName+".md")
			if _, err := os.Stat(fullpath); err == nil {
				// If file already exists, append a number to the filename
				cardNumber++
				fullpath = filepath.Join(cardPath, checklistName+" "+strconv.Itoa(cardNumber)+".md")
			}

			fmt.Println("Creating checklist markdown file:", fullpath)
			// Create markdown file for card checklists
			err = os.WriteFile(fullpath, buff.Bytes(), 0644)
			if err != nil {
				panic(err)
			}
		}

		/*
			In Card Directory store:
				~~ Card Description markdown
				~~ Card Checklist markdown (including properly checked items)
				~~ Directory called attachments that stores:
					~~ File attachments (tag cover photo in name)
					~~ Links
				Card Checklists into markdown file
				Card Comments into markdown file
				Card Users into markdown file
				Card Labels markdown file
				Card History in markdown file
		*/
	}
}
