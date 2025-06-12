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

	/*		Get Board Members
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
					filePath := filepath.Join(cardPath, "attachments")
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
			// Create an empty attachments directory if no attachments found
			dirCreate(cardPath + "/attachments")
		}

		/*
			Save Card Checklists
			- Create markdown file for each checklist
			- Include checked items in markdown
		*/
		cardNumber = 0
		fmt.Println("Found", len(card.IDCheckLists), "checklists for card", card.Name)

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
			fmt.Println("Processing checklist:", checklistName)

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

			fmt.Println("Creating checklist markdown file:", fullpath)
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
		fmt.Println("Grabbing comments for card:", card.Name)
		comments, err := card.GetActions(trello.Arguments{"filter": "commentCard"})
		if err != nil {
			fmt.Println("Error: Unable to get comments for card ID", card.ID)
			continue
		}
		if len(comments) > 0 {
			// Clear the old Bytes Buffer
			buff.Reset()
			fmt.Println("Found", len(comments), "comments for card", card.Name)
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
			fmt.Println("Created comments markdown file:", commentFileName)
		} else {
			fmt.Println("No comments found on card", card.Name)
			// Create an empty comments markdown file if no comments found
			// This is to ensure the file exists for future reference
			commentFileName := cardPath + "/CardComments.md"
			_ = os.WriteFile(commentFileName, nil, 0644)
		}

		/*
			Save Card Users
		*/
		fmt.Println("Grabbing users for card:", card.Name)
		members, err := card.GetMembers()
		if err != nil {
			fmt.Println("Error: Unable to get members for card ID", card.ID)
			continue
		}

		if len(members) > 0 {
			// Clear the old Bytes Buffer
			buff.Reset()
			fmt.Println("Found", len(members), "members for card", card.Name)
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
			fmt.Println("Created users markdown file:", userFileName)
		} else {
			fmt.Println("No users found on card", card.Name)
			// Create an empty users markdown file if no users found
			// This is to ensure the file exists for future reference
			userFileName := cardPath + "/CardUsers.md"
			_ = os.WriteFile(userFileName, nil, 0644)
		}

		/*
			Save Card Labels
		*/
		fmt.Println("Grabbing labels for card:", card.Name)
		cardWithLabels, err := client.GetCard(card.ID, trello.Arguments{"labels": "all"})
		if err != nil {
			fmt.Println("Error: Unable to get labels for card ID", card.ID)
			continue
		}
		if len(cardWithLabels.Labels) > 0 {
			// Clear the old Bytes Buffer
			buff.Reset()
			fmt.Println("Found", len(cardWithLabels.Labels), "labels for card", card.Name)
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
			fmt.Println("Created labels markdown file:", labelFileName)
		} else {
			fmt.Println("No labels found on card", card.Name)
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
		fmt.Println("Grabbing history for card:", card.Name)
		history, err := card.GetActions(trello.Arguments{"filter": "all"})
		if err != nil {
			fmt.Println("Error: Unable to get history for card ID", card.ID)
			continue
		}
		if len(history) > 0 {
			// Clear the old Bytes Buffer
			buff.Reset()
			fmt.Println("Found", len(history), "history actions for card", card.Name)
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
			fmt.Println("Created history markdown file:", historyFileName)
		} else {
			fmt.Println("No history found for card", card.Name)
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
			fmt.Println("Created due date markdown file:", dueFileName)
		} else {
			fmt.Println("No due date found for card", card.Name)
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
			fmt.Println("Created start date markdown file:", startFileName)
		} else {
			fmt.Println("No start date found for card", card.Name)
			// Create an empty start date markdown file if no start date found
			// This is to ensure the file exists for future reference
			startFileName := cardPath + "/CardStartDate.md"
			_ = os.WriteFile(startFileName, nil, 0644)
		}

		/*			Save Card Cover Image
					- Download cover image if exists
					- Save it in the card directory
					- If cover is a color, save as a text file with the color name
		*/
		if card.Cover == nil {
			fmt.Println("No cover set on card", card.Name)
			return
		}

		cover := card.Cover

		// IMAGE COVER: Trello will populate IDAttachment (or IDUploadedBackground)
		//   if there’s an image; the Scaled slice contains URLs for each size variant.
		if cover.IDAttachment != "" || cover.IDUploadedBackground != "" {
			// pick a URL from the Scaled variants. Prefer the un-scaled (original) if it exists
			var imgURL string
			for _, v := range cover.Scaled {
				if !v.Scaled {
					imgURL = v.URL
					break
				}
			}
			if imgURL == "" && len(cover.Scaled) > 0 {
				// fallback to the first one if no original variant
				imgURL = cover.Scaled[0].URL
			}

			if imgURL == "" {
				fmt.Println("no scaled image URL available for cover on", card.Name)
			} else {
				fmt.Println("Downloading cover image for", card.Name, "from", imgURL)
				if err := downLoadFile(imgURL, filepath.Join(cardPath, "CardCover.jpg")); err != nil {
					fmt.Println("download error:", err)
				}
			}

			// COLORED COVER: if Color is non-empty you have a solid cover color
		} else if cover.Color != "" {
			colorFile := filepath.Join(cardPath, "CardCoverColor.md")
			if err := os.WriteFile(colorFile, []byte(cover.Color), 0644); err != nil {
				fmt.Println("Error writing cover color for", card.Name, ":", err)
			}
		}

		/* ##### SEE CHATGPT DISCUSSION ON THIS, TO TRY NEXT #####
		/*if card.Cover != nil {
			if card.Cover.IsImage() {
				// Download cover image
				url := card.Cover.GetImageURL()
				localFilePath := filepath.Join(cardPath, "CardCover-")
				err := downLoadFile(url, localFilePath)
				if err != nil {
					fmt.Println("Error: Unable to download cover image for card", card.Name)
					fmt.Println(err)
				}
			} else if card.Cover.IsColor() {
				// Save cover color as a text file
				colorFileName := filepath.Join(cardPath, "CardCoverColor.md")
				err := os.WriteFile(colorFileName, []byte(card.Cover.Color), 0644)
				if err != nil {
					fmt.Println("Error: Unable to save cover color for card", card.Name)
					fmt.Println(err)
				}
			}
		} else {
			fmt.Println("No cover found for card", card.Name)
		}*/
	}
}
