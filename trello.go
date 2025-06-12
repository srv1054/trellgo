package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/adlio/trello"
)

/*
dumpABoard - Process the board data and dump to specified directory structure

	assumes board ID is valid and exists
	assumes client is already authenticated with valid API key and token
*/
func dumpABoard(config Config, board *trello.Board, client *trello.Client) {

	var (
		cards         []*trello.Card
		err           error
		cleanListPath string
		cleanCardPath string
		cardPath      string
		dueFileName   string
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
		localFilePath := filepath.Join(config.ARGS.StoragePath, board.Name, "BoardBackground-")
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
		labelFileName := filepath.Join(config.ARGS.StoragePath, board.Name, "BoardLabels.md")
		err := os.WriteFile(labelFileName, buf.Bytes(), 0644)
		if err != nil {
			panic(err)
		}
	}

	/*
		Get Board Members
		- Create markdown file for board members
		- Include member name and ID
	*/
	fmt.Println("Grabbing members for board:", board.Name)
	members, err := board.GetMembers()
	if err != nil {
		fmt.Println("Error: Unable to get members for board ID", board.ID)
	} else {
		var memberBuf bytes.Buffer
		for _, member := range members {
			memberBuf.WriteString(fmt.Sprintf("**%s** (%s)\n", member.FullName, member.ID))
		}
		// Write buffer content to a file
		memberFileName := filepath.Join(config.ARGS.StoragePath, board.Name, "BoardMembers.md")
		err := os.WriteFile(memberFileName, memberBuf.Bytes(), 0644)
		if err != nil {
			panic(err)
		}
	}

	/*
		Get all cards
		- If -a flag is set, include archived cards
		- If -l flag is set, only include cards with the specified label NAME (not Label ID)
			- Does not work with -a flag
		- If -split flag is set, archived cards will be moved to an ARCHIVED directory
	*/
	// Handle specific label ID search, if provided (-l flag)
	if config.ARGS.LabelID != "" {
		fmt.Println("Searching for only cards with label ID:", config.ARGS.LabelID)
		query := fmt.Sprintf("board:%s label:\"%s\" is:open", board.ID, config.ARGS.LabelID)
		if ListLoud {
			fmt.Println("Querying Trello API with:", query)
		}
		cards, err = client.SearchCards(query, trello.Defaults())
		if err != nil {
			fmt.Println("Error: Unable to get card data for board ID", config.ARGS.BoardID, "with label ID", config.ARGS.LabelID)
			os.Exit(1)
		}
	} else {
		// If no specific label ID is provided, get all cards based on the -a flag
		if config.ARGS.Archived {
			cards, err = board.GetCards(trello.Arguments{"filter": "all"})
		} else {
			cards, err = board.GetCards(trello.Arguments{"filter": "open"})
		}
		if err != nil {
			fmt.Println("Error: Unable to get card data for board ID", config.ARGS.BoardID)
			os.Exit(1)
		}
	}

	// If no cards found, exit with message
	if len(cards) == 0 {
		fmt.Println("No cards found for board", board.Name)
		os.Exit(0)
	} else {
		if len(cards) > 1 {
			fmt.Printf("Found %d cards on board %s\n. Processing Cards, please wait...", len(cards), board.Name)
		} else {
			fmt.Printf("Found %d card on board %s\n. Processing this card, please wait...", len(cards), board.Name)
		}
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
		dirCreate(filepath.Join(config.ARGS.StoragePath, board.Name, cleanListPath))

		// Create directory for card name
		cleanCardPath = SanitizePathName(card.Name)
		// If card is archived, append ARCHIVED to the card name or move to ARCHIVED directory
		if card.Closed {
			if !config.ARGS.SeparateArchived {
				// If -split flag is not set, append ARCHIVED to the card name
				cardPath = filepath.Join(config.ARGS.StoragePath, board.Name, cleanListPath, cleanCardPath+" (ARCHIVED)")
				// If -split flag is set, move to ARCHIVED directory
			} else {
				cardPath = filepath.Join(config.ARGS.StoragePath, board.Name, "ARCHIVED", cleanListPath, cleanCardPath)
			}
			// card is not archived
		} else {
			cardPath = filepath.Join(config.ARGS.StoragePath, board.Name, cleanListPath, cleanCardPath)
		}

		dirCreate(cardPath)

		/*
			Card Level Data
		*/
		if ListLoud {
			fmt.Println("Dumping card:", card.Name)
		}
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
			if ListLoud {
				fmt.Println(card.Name + " has  " + strconv.Itoa(len(attachments)) + " attachments")
			}

			for _, a := range attachments {

				if a.IsUpload {
					// Download
					filePath := filepath.Join(cardPath, "attachments")
					if card.Cover.IDAttachment == a.ID {
						// If this is the cover attachment, append "Cover" to the filename
						filePath = filepath.Join(filePath, a.Name+" (Card Cover)")
						if ListLoud {
							fmt.Println("This is the cover attachment for card", card.Name, "downloading to", filePath)
						}
					} else {
						filePath = filepath.Join(filePath, a.Name)
					}
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
			if ListLoud {
				fmt.Println("No attachments found for card", card.Name)
			}
			// Create an empty attachments directory if no attachments found
			dirCreate(cardPath + "/attachments")
		}

		/*
			Save Card Checklists
			- Create markdown file for each checklist
			- Include checked items in markdown
		*/
		cardNumber = 0
		if ListLoud {
			fmt.Println("Found", len(card.IDCheckLists), "checklists for card", card.Name)
		}

		dirCreate(cardPath + "/checklists")

		for _, checkList := range card.IDCheckLists {
			// Clear the old Bytes Buffer
			buff.Reset()

			// Get checklist data
			args := trello.Arguments{"checkItems": "all"}
			checklist, err := client.GetChecklist(checkList, args)
			if err != nil {
				fmt.Println("Error: Unable to get checklist data for checklist ID", checkList)
				continue
			}

			checklistName := SanitizePathName(checklist.Name)
			if ListLoud {
				fmt.Println("Processing checklist:", checklistName)
			}

			for _, item := range checklist.CheckItems {
				// If item is checked, append [x] to the name, otherwise append [ ]
				if item.State == "complete" {
					buff.WriteString(fmt.Sprintf("- [x] %s\n", item.Name))
				} else {
					buff.WriteString(fmt.Sprintf("- [ ] %s\n", item.Name))
				}
			}

			fullpath := filepath.Join(cardPath, "checklists", checklistName+".md")
			if _, err := os.Stat(fullpath); err == nil {
				// If file already exists, append a number to the filename
				cardNumber++
				fullpath = filepath.Join(cardPath, checklistName+" "+strconv.Itoa(cardNumber)+".md")
			}

			if ListLoud {
				fmt.Println("Creating checklist markdown file:", fullpath)
			}
			// Create markdown file for card checklists
			err = os.WriteFile(fullpath, buff.Bytes(), 0644)
			if err != nil {
				panic(err)
			}
		}

		/*
			Save Card Comments
			- Create markdown file for card comments
			- Include comment author and date
		*/
		if ListLoud {
			fmt.Println("Grabbing comments for card:", card.Name)
		}
		comments, err := card.GetActions(trello.Arguments{"filter": "commentCard"})
		if err != nil {
			fmt.Println("Error: Unable to get comments for card ID", card.ID)
			continue
		}
		if len(comments) > 0 {
			// Clear the old Bytes Buffer
			buff.Reset()
			if ListLoud {
				fmt.Println("Found", len(comments), "comments for card", card.Name)
			}
			for _, comment := range comments {
				// Format comment with author and date
				buff.WriteString(fmt.Sprintf("**%s** (%s): %s\n", comment.MemberCreator.FullName, comment.Date.Format("2006-01-02 15:04:05"), comment.Data.Text))
			}
			// Create markdown file for card comments
			commentFileName := cardPath + "/CardComments.md"
			err = os.WriteFile(commentFileName, buff.Bytes(), 0644)
			if err != nil {
				panic(err)
			}
			if ListLoud {
				fmt.Println("Created comments markdown file:", commentFileName)
			}
		} else {
			if ListLoud {
				fmt.Println("No comments found on card", card.Name)
			}
			// Create an empty comments markdown file if no comments found
			// This is to ensure the file exists for future reference
			commentFileName := cardPath + "/CardComments.md"
			_ = os.WriteFile(commentFileName, nil, 0644)
		}

		/*
			Save Card Users
		*/
		if ListLoud {
			fmt.Println("Grabbing users for card:", card.Name)
		}
		members, err := card.GetMembers()
		if err != nil {
			fmt.Println("Error: Unable to get members for card ID", card.ID)
			continue
		}

		if len(members) > 0 {
			// Clear the old Bytes Buffer
			buff.Reset()
			if ListLoud {
				fmt.Println("Found", len(members), "members for card", card.Name)
			}
			for _, member := range members {
				// Format member with name and ID
				buff.WriteString(fmt.Sprintf("**%s** (%s)\n", member.FullName, member.ID))
			}
			// Create markdown file for card users
			userFileName := cardPath + "/CardUsers.md"
			err = os.WriteFile(userFileName, buff.Bytes(), 0644)
			if err != nil {
				panic(err)
			}
			if ListLoud {
				fmt.Println("Created users markdown file:", userFileName)
			}
		} else {
			if ListLoud {
				fmt.Println("No users found on card", card.Name)
			}
			// Create an empty users markdown file if no users found
			// This is to ensure the file exists for future reference
			userFileName := cardPath + "/CardUsers.md"
			_ = os.WriteFile(userFileName, nil, 0644)
		}

		/*
			Save Card Labels
		*/
		if ListLoud {
			fmt.Println("Grabbing labels for card:", card.Name)
		}
		cardWithLabels, err := client.GetCard(card.ID, trello.Arguments{"labels": "all"})
		if err != nil {
			fmt.Println("Error: Unable to get labels for card ID", card.ID)
			continue
		}
		if len(cardWithLabels.Labels) > 0 {
			// Clear the old Bytes Buffer
			buff.Reset()
			if ListLoud {
				fmt.Println("Found", len(cardWithLabels.Labels), "labels for card", card.Name)
			}
			for _, label := range cardWithLabels.Labels {
				// Format label with name and ID
				buff.WriteString(fmt.Sprintf("**%s** - %s (%s)\n", label.Name, label.Color, label.ID))
			}
			// Create markdown file for card labels
			labelFileName := cardPath + "/CardLabels.md"
			err = os.WriteFile(labelFileName, buff.Bytes(), 0644)
			if err != nil {
				panic(err)
			}
			if ListLoud {
				fmt.Println("Created labels markdown file:", labelFileName)
			}
		} else {
			if ListLoud {
				fmt.Println("No labels found on card", card.Name)
			}
			// Create an empty labels markdown file if no labels found
			// This is to ensure the file exists for future reference
			labelFileName := cardPath + "/CardLabels.md"
			_ = os.WriteFile(labelFileName, nil, 0644)
		}

		/*
			Save Card History
			- Create markdown file for card history
			- Include action type, date, and member who performed the action
		*/
		if ListLoud {
			fmt.Println("Grabbing history for card:", card.Name)
		}
		history, err := card.GetActions(trello.Arguments{"filter": "all"})
		if err != nil {
			fmt.Println("Error: Unable to get history for card ID", card.ID)
			continue
		}
		if len(history) > 0 {
			// Clear the old Bytes Buffer
			buff.Reset()
			if ListLoud {
				fmt.Println("Found", len(history), "history actions for card", card.Name)
			}
			for _, action := range history {
				// Format action with type, date, and member
				buff.WriteString(fmt.Sprintf("**%s** (%s): %s - %s\n", action.Type, action.Date.Format("2006-01-02 15:04:05"), action.MemberCreator.FullName, action.Data.Text))
			}
			// Create markdown file for card history
			historyFileName := cardPath + "/CardHistory.md"
			err = os.WriteFile(historyFileName, buff.Bytes(), 0644)
			if err != nil {
				panic(err)
			}
			if ListLoud {
				fmt.Println("Created history markdown file:", historyFileName)
			}
		} else {
			if ListLoud {
				fmt.Println("No history found for card", card.Name)
			}
			// Create an empty history markdown file if no history found
			// This is to ensure the file exists for future reference
			historyFileName := cardPath + "/CardHistory.md"
			_ = os.WriteFile(historyFileName, nil, 0644)
		}

		/*
			Save Card Due Date
			- Create markdown file for card due date
		*/
		if card.Due != nil {
			if card.DueComplete {
				dueFileName = cardPath + "/CardDueDate (Completed).md"
			} else {
				dueFileName = cardPath + "/CardDueDate.md"
			}
			err := os.WriteFile(dueFileName, []byte(card.Due.Format("2006-01-02 15:04:05")), 0644)

			if err != nil {
				panic(err)
			}
			if ListLoud {
				fmt.Println("Created due date markdown file:", dueFileName)
			}
		} else {
			if ListLoud {
				fmt.Println("No due date found for card", card.Name)
			}
			// Create an empty due date markdown file if no due date found
			// This is to ensure the file exists for future reference
			dueFileName := cardPath + "/CardDueDate.md"
			_ = os.WriteFile(dueFileName, nil, 0644)
		}

		/*
			Save Card Start Date
			- Create markdown file for card start date
		*/
		if card.Start != nil {
			startFileName := cardPath + "/CardStartDate.md"
			err := os.WriteFile(startFileName, []byte(card.Start.Format("2006-01-02 15:04:05")), 0644)

			if err != nil {
				panic(err)
			}
			if ListLoud {
				fmt.Println("Created start date markdown file:", startFileName)
			}
		} else {
			if ListLoud {
				fmt.Println("No start date found for card", card.Name)
			}
			// Create an empty start date markdown file if no start date found
			// This is to ensure the file exists for future reference
			startFileName := cardPath + "/CardStartDate.md"
			_ = os.WriteFile(startFileName, nil, 0644)
		}

		/*
			Save Card Cover
			- If cover is an image, it was already downloaded in the attachments section and labeled with (Card Cover)
			- If cover is a color, save as a markdown file with the color name
		*/
		if card.Cover == nil {
			if ListLoud {
				fmt.Println("No cover set on card", card.Name)
			}
		} else if card.Cover.Color != "" {
			colorFile := filepath.Join(cardPath, "CardCoverColor.md")
			if err := os.WriteFile(colorFile, []byte(card.Cover.Color), 0644); err != nil {
				fmt.Println("Error writing cover color for", card.Name, ":", err)
			}
		} else {
			if ListLoud {
				fmt.Println("Cover is an image, already downloaded in attachments for card", card.Name)
			}
		}

	}
}
