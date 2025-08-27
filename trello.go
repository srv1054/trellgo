package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/adlio/trello"
)

// Secure file permissions - owner read/write only
const (
	SecureFileMode = 0600 // Owner read/write only
	MaxWorkers     = 5    // Maximum concurrent workers for card processing
)

// CardProcessingJob represents work to be done by a worker
type CardProcessingJob struct {
	card      *trello.Card
	board     *trello.Board
	boardPath string
	config    Config
	client    *trello.Client
	listCache map[string]*trello.List
	index     int
	total     int
}

// Buffer pool for reusing byte buffers across concurrent workers
var bufferPool = sync.Pool{
	New: func() interface{} {
		return &bytes.Buffer{}
	},
}

// getBuffer gets a clean buffer from the pool
func getBuffer() *bytes.Buffer {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset() // Ensure buffer is clean
	return buf
}

// putBuffer returns a buffer to the pool for reuse
func putBuffer(buf *bytes.Buffer) {
	// Don't return huge buffers to the pool to avoid memory bloat
	if buf.Cap() > 64*1024 { // 64KB limit
		return
	}
	bufferPool.Put(buf)
}

/*
processCardWorker processes individual cards concurrently
*/
func processCardWorker(jobs <-chan CardProcessingJob, results chan<- error, processed *int64) {
	for job := range jobs {
		err := processSingleCard(job)
		if err != nil {
			results <- err
		} else {
			results <- nil
		}

		// Update progress counter atomically
		current := atomic.AddInt64(processed, 1)

		// Show progress
		if !ListLoud && !job.config.ARGS.SuperQuiet {
			fmt.Printf("\rProcessing %3d/%3d", current, job.total)
		}
	}
}

/*
processSingleCard handles the processing of a single card
Allows concurrent processing
*/
func processSingleCard(job CardProcessingJob) error {
	var (
		cleanListPath string
		cleanCardPath string
		cardPath      string
		dueFileName   string
		cardNumber    int
	)

	// Get buffer from pool instead of allocating new one
	buff := getBuffer()
	defer putBuffer(buff) // Return buffer to pool when done

	card := job.card
	config := job.config
	client := job.client
	boardPath := job.boardPath
	listCache := job.listCache

	// Use cached list instead of API call
	list, exists := listCache[card.IDList]
	if !exists {
		// Fallback to API call if not in cache
		var err error
		list, err = client.GetList(card.IDList, trello.Defaults())
		if err != nil {
			return handleProcessingError(
				newProcessingError("get list data", fmt.Sprintf("list ID %s", card.IDList), ErrorSeverityCritical, err),
				config)
		}
	}

	// create list directory
	cleanListPath = SanitizePathName(list.Name)
	dirCreate(filepath.Join(config.ARGS.StoragePath, boardPath, cleanListPath))

	// We need to handle when card is a LINK and not a regular card
	// Trello Go client does not support the new field `cardRole` so we have to do our own thing here for now.  6/16/2025
	isCardLink, _ := isLinkCard(client, card.ID)

	if isCardLink {
		return processLinkCard(card, config, boardPath, cleanListPath)
	}

	// Get comprehensive card data in one API call instead of multiple calls
	comprehensiveCard, err := getComprehensiveCardData(card.ID, client)
	if err != nil {
		logger("Warning: Failed to get comprehensive card data, falling back to individual calls: "+err.Error(), "warn", true, true, config)
		comprehensiveCard = card // Fallback to original card
	}

	// Process regular card with comprehensive data
	return processRegularCard(comprehensiveCard, config, client, boardPath, cleanListPath, buff, &cardNumber, &dueFileName, &cleanCardPath, &cardPath)
}

/*
processLinkCard handles processing of Trello link cards
*/
func processLinkCard(card *trello.Card, config Config, boardPath, cleanListPath string) error {
	// We should dump this into their own directory as they can be messy filenames
	logger("This card is a link file only, processing as .MD instead of directory", "info", true, true, config)
	thisCardLinkPath := filepath.Join(config.ARGS.StoragePath, boardPath, cleanListPath, "Link Cards Only")
	dirCreate(thisCardLinkPath)
	logger("Created Custom Directory for Link Cards: "+thisCardLinkPath, "info", true, true, config)
	// Cleanup messy filename
	cleanName := SanitizePathName(card.Name)
	cleanName = strings.ReplaceAll(cleanName, "https---", "")
	cleanName = strings.ReplaceAll(cleanName, "http---", "")
	cleanName = "CARD - " + cleanName + ".md"
	logger("New Clean Custom Card File Name: "+cleanName, "info", true, true, config)
	thisCardPath := filepath.Join(thisCardLinkPath, cleanName)
	// Dump URL into card md file
	err := os.WriteFile(thisCardPath, []byte(card.Name), SecureFileMode)
	if err != nil {
		logger("CRITICAL - Unable to write buffer to file for "+thisCardPath+" Error: "+err.Error(), "err", true, true, config)
		errorWarnOnCompletion = true
		return err
	}
	return nil
}

/*
processRegularCard handles processing of regular Trello cards with all their data
*/
func processRegularCard(card *trello.Card, config Config, client *trello.Client, boardPath, cleanListPath string,
	buff *bytes.Buffer, cardNumber *int, dueFileName *string, cleanCardPath *string, cardPath *string) error {

	// Create directory for card name
	*cleanCardPath = SanitizePathName(card.Name)
	// If card is archived, append ARCHIVED to the card name or move to ARCHIVED directory
	if card.Closed {
		if !config.ARGS.SeparateArchived {
			// If -split flag is not set, append ARCHIVED to the card name
			*cardPath = filepath.Join(config.ARGS.StoragePath, boardPath, cleanListPath, *cleanCardPath+" (ARCHIVED)")
			// If -split flag is set, move to ARCHIVED directory
		} else {
			*cardPath = filepath.Join(config.ARGS.StoragePath, boardPath, "ARCHIVED", cleanListPath, *cleanCardPath)
		}
		// card is not archived
	} else {
		*cardPath = filepath.Join(config.ARGS.StoragePath, boardPath, cleanListPath, *cleanCardPath)
	}

	dirCreate(*cardPath)

	// Process all card data
	if err := processCardDescription(card, *cardPath, config); err != nil {
		return err
	}
	if err := processCardAttachments(card, *cardPath, config, buff); err != nil {
		return err
	}
	if err := processCardChecklists(card, client, *cardPath, config, buff, cardNumber); err != nil {
		return err
	}
	if err := processCardComments(card, *cardPath, config, buff); err != nil {
		return err
	}
	if err := processCardUsers(card, *cardPath, config, buff); err != nil {
		return err
	}
	if err := processCardLabels(card, client, *cardPath, config, buff); err != nil {
		return err
	}
	if err := processCardHistory(card, *cardPath, config, buff); err != nil {
		return err
	}
	if err := processCardDates(card, *cardPath, config, dueFileName); err != nil {
		return err
	}
	if err := processCardCover(card, *cardPath, config); err != nil {
		return err
	}

	return nil
}

/*
processCardDescription creates markdown file for card description
*/
func processCardDescription(card *trello.Card, cardPath string, config Config) error {
	logger("Dumping card: "+card.Name, "info", true, true, config)
	err := os.WriteFile(filepath.Join(cardPath, "CardDescription.md"), []byte(card.Desc), SecureFileMode)
	if err != nil {
		return handleProcessingError(
			newProcessingError("write card description", fmt.Sprintf("card %s", card.Name), ErrorSeverityCritical, err),
			config)
	}
	return nil
}

/*
processCardAttachments downloads file attachments and saves URL attachments
Uses comprehensive card data instead of additional API call
*/
func processCardAttachments(card *trello.Card, cardPath string, config Config, buff *bytes.Buffer) error {
	// PERFORMANCE: Use attachments from comprehensive card data instead of API call
	attachments := card.Attachments
	if attachments == nil {
		// Fallback to API call if not available in comprehensive data
		var err error
		attachments, err = card.GetAttachments(trello.Defaults())
		if err != nil {
			return handleProcessingError(
				newProcessingError("get attachments", fmt.Sprintf("card %s", card.Name), ErrorSeverityWarning, err),
				config)
		}
	}

	// Clear the old Bytes Buffer
	buff.Reset()

	if len(attachments) > 0 {
		dirCreate(filepath.Join(cardPath, "attachments"))
		logger(card.Name+" has "+strconv.Itoa(len(attachments))+" attachments", "info", true, true, config)

		for _, a := range attachments {
			if a == nil {
				continue
			}

			if a.IsUpload {
				// Download
				filePath := filepath.Join(cardPath, "attachments")
				if card.Cover != nil && card.Cover.IDAttachment == a.ID {
					// If this is the cover attachment, append "Cover" to the filename
					filePath = filepath.Join(filePath, a.Name+" (Card Cover)")
					logger("This is the cover attachment for card "+card.Name+" downloading to "+filePath, "info", true, true, config)
				} else {
					filePath = filepath.Join(filePath, a.Name)
				}
				// Format https://api.trello.com/1/cards/{idCard}/attachments/{idAttachment}/download/{attachmentFileName}
				authURL := fmt.Sprintf("https://api.trello.com/1/cards/%s/attachments/%s/download/%s", card.ID, a.ID, a.Name)
				err := downloadFileAuthHeader(authURL, filePath, config.ENV.TRELLOAPIKEY, config.ENV.TRELLOAPITOK)
				if err != nil {
					logger("Error downloading attachment from "+sanitizeURLForLogging(authURL)+" to "+filePath+": "+err.Error(), "err", true, false, config)
				}
			} else {
				// build a bytes.buffer for URL attachments
				buff.WriteString(a.URL)
				buff.WriteString("\n")
			}
		}

		// Write buffer to disc for URL Attachments
		err := os.WriteFile(filepath.Join(cardPath, "attachments", "URL-Attachments.md"), buff.Bytes(), SecureFileMode)
		if err != nil {
			return handleProcessingError(
				newProcessingError("write URL attachments", fmt.Sprintf("card %s", card.Name), ErrorSeverityCritical, err),
				config)
		}
	} else {
		logger("No attachments found for card "+card.Name, "warn", true, true, config)
		// Create an empty attachments directory if no attachments found
		dirCreate(filepath.Join(cardPath, "attachments"))
	}
	return nil
}

/*
processCardChecklists creates markdown files for each checklist
*/
func processCardChecklists(card *trello.Card, client *trello.Client, cardPath string, config Config, buff *bytes.Buffer, cardNumber *int) error {
	*cardNumber = 0
	logger("Found "+strconv.Itoa(len(card.IDCheckLists))+" checklists for card "+card.Name, "info", true, true, config)

	dirCreate(filepath.Join(cardPath, "checklists"))

	for _, checkList := range card.IDCheckLists {
		if checkList == "" {
			continue
		}
		// Clear the old Bytes Buffer
		buff.Reset()

		// Get checklist data
		args := trello.Arguments{"checkItems": "all"}
		checklist, err := client.GetChecklist(checkList, args)
		if err != nil {
			logger("Error: Unable to get checklist data for checklist ID "+checkList, "err", true, false, config)
			continue
		}

		checklistName := SanitizePathName(checklist.Name)
		logger("Processing checklist: "+checklistName, "info", true, true, config)

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
			*cardNumber++
			fullpath = filepath.Join(cardPath, "checklists", checklistName+" "+strconv.Itoa(*cardNumber)+".md")
		}

		logger("Creating checklist markdown file: "+fullpath, "info", true, true, config)

		// Create markdown file for card checklists
		err = os.WriteFile(fullpath, buff.Bytes(), SecureFileMode)
		if err != nil {
			logger("CRITICAL - Unable to write buffer to file for "+fullpath+" Error: "+err.Error(), "err", true, true, config)
			errorWarnOnCompletion = true
			return err
		}
	}
	return nil
}

/*
processCardComments creates markdown file for card comments
Uses comprehensive card data instead of additional API call
*/
func processCardComments(card *trello.Card, cardPath string, config Config, buff *bytes.Buffer) error {
	logger("Grabbing comments for card: "+card.Name, "info", true, true, config)

	// Filter comments from comprehensive card actions instead of API call
	var comments []*trello.Action
	if card.Actions != nil {
		for _, action := range card.Actions {
			if action != nil && action.Type == "commentCard" {
				comments = append(comments, action)
			}
		}
	} else {
		// Fallback to API call if actions not available in comprehensive data
		var err error
		comments, err = card.GetActions(trello.Arguments{"filter": "commentCard"})
		if err != nil {
			logger("Error: Unable to get comments for card ID "+card.ID, "err", true, false, config)
			return nil // Don't fail the entire card for comment errors
		}
	}

	commentFileName := filepath.Join(cardPath, "CardComments.md")
	if len(comments) > 0 {
		// Clear the old Bytes Buffer
		buff.Reset()
		logger("Found "+strconv.Itoa(len(comments))+" comments for card "+card.Name, "info", true, true, config)
		for _, comment := range comments {
			if comment.MemberCreator == nil || comment.MemberCreator.FullName == "" {
				comment.MemberCreator = &trello.Member{FullName: "Unknown Member"}
			}
			// Format comment with author and date
			buff.WriteString(fmt.Sprintf("**%s** (%s): %s\n", comment.MemberCreator.FullName, comment.Date.Format("2006-01-02 15:04:05"), comment.Data.Text))
		}
		// Create markdown file for card comments
		err := os.WriteFile(commentFileName, buff.Bytes(), SecureFileMode)
		if err != nil {
			logger("CRITICAL - Unable to write buffer to file for "+commentFileName+" Error: "+err.Error(), "err", true, true, config)
			errorWarnOnCompletion = true
			return err
		}
		logger("Created comments markdown file: "+commentFileName, "info", true, true, config)
	} else {
		logger("No comments found on card "+card.Name, "warn", true, true, config)
		// Create an empty comments markdown file if no comments found
		_ = os.WriteFile(commentFileName, nil, SecureFileMode)
	}
	return nil
}

/*
processCardUsers creates markdown file for card users/members
Uses comprehensive card data instead of additional API call
*/
func processCardUsers(card *trello.Card, cardPath string, config Config, buff *bytes.Buffer) error {
	logger("Grabbing users for card: "+card.Name, "info", true, true, config)

	// Use members from comprehensive card data instead of API call
	members := card.Members
	if members == nil {
		// Fallback to API call if not available in comprehensive data
		var err error
		members, err = card.GetMembers()
		if err != nil {
			logger("Error: Unable to get members for card ID "+card.ID, "err", true, false, config)
			return nil // Don't fail the entire card for member errors
		}
	}

	userFileName := filepath.Join(cardPath, "CardUsers.md")
	if len(members) > 0 {
		// Clear the old Bytes Buffer
		buff.Reset()
		logger("Found "+strconv.Itoa(len(members))+" members for card "+card.Name, "info", true, true, config)
		for _, member := range members {
			if member == nil || member.FullName == "" {
				member = &trello.Member{FullName: "Unknown Member", ID: "Unknown ID"}
			}
			// Format member with name and ID
			buff.WriteString(fmt.Sprintf("**%s** (%s)\n", member.FullName, member.ID))
		}
		// Create markdown file for card users
		err := os.WriteFile(userFileName, buff.Bytes(), SecureFileMode)
		if err != nil {
			logger("CRITICAL - Unable to write buffer to file for "+userFileName+" Error: "+err.Error(), "err", true, true, config)
			errorWarnOnCompletion = true
			return err
		}
		logger("Created users markdown file: "+userFileName, "info", true, true, config)
	} else {
		logger("No users found on card "+card.Name, "warn", true, true, config)
		// Create an empty users markdown file if no users found
		_ = os.WriteFile(userFileName, nil, SecureFileMode)
	}
	return nil
}

/*
processCardLabels creates markdown file for card labels
Uses comprehensive card data instead of additional API call
*/
func processCardLabels(card *trello.Card, client *trello.Client, cardPath string, config Config, buff *bytes.Buffer) error {
	logger("Grabbing labels for card: "+card.Name, "info", true, true, config)

	// PERFORMANCE: Use labels from comprehensive card data instead of additional API call
	labels := card.Labels
	if labels == nil {
		// Fallback to API call if not available in comprehensive data
		cardWithLabels, err := client.GetCard(card.ID, trello.Arguments{"labels": "all"})
		if err != nil {
			logger("Error: Unable to get labels for card ID "+card.ID, "err", true, false, config)
			return nil // Don't fail the entire card for label errors
		}
		labels = cardWithLabels.Labels
	}

	labelFileName := filepath.Join(cardPath, "CardLabels.md")
	if len(labels) > 0 {
		// Clear the old Bytes Buffer
		buff.Reset()
		logger("Found "+strconv.Itoa(len(labels))+" labels for card "+card.Name, "info", true, true, config)
		for _, label := range labels {
			if label == nil {
				continue
			}
			// Format label with name and ID
			buff.WriteString(fmt.Sprintf("**%s** - %s (%s)\n", label.Name, label.Color, label.ID))
		}
		// Create markdown file for card labels
		err := os.WriteFile(labelFileName, buff.Bytes(), SecureFileMode)
		if err != nil {
			logger("CRITICAL - Unable to write buffer to file for "+labelFileName+" Error: "+err.Error(), "err", true, true, config)
			errorWarnOnCompletion = true
			return err
		}
		logger("Created labels markdown file: "+labelFileName, "info", true, true, config)
	} else {
		logger("No labels found on card "+card.Name, "warn", true, true, config)
		// Create an empty labels markdown file if no labels found
		_ = os.WriteFile(labelFileName, nil, SecureFileMode)
	}
	return nil
}

/*
processCardHistory creates markdown file for card history/actions
Uses comprehensive card data instead of additional API call
*/
func processCardHistory(card *trello.Card, cardPath string, config Config, buff *bytes.Buffer) error {
	logger("Grabbing history for card: "+card.Name, "info", true, true, config)

	// Use actions from comprehensive card data instead of API call
	history := card.Actions
	if history == nil {
		// Fallback to API call if not available in comprehensive data
		var err error
		history, err = card.GetActions(trello.Arguments{"filter": "all"})
		if err != nil {
			logger("Error: Unable to get history for card ID "+card.ID, "err", true, true, config)
			return nil // Don't fail the entire card for history errors
		}
	}

	historyFileName := filepath.Join(cardPath, "CardHistory.md")
	if len(history) > 0 {
		// Clear the old Bytes Buffer
		buff.Reset()
		logger("Found "+strconv.Itoa(len(history))+" history actions for card "+card.Name, "info", true, true, config)
		for _, action := range history {
			if action == nil {
				continue
			}
			if action.MemberCreator == nil || action.MemberCreator.FullName == "" {
				action.MemberCreator = &trello.Member{FullName: "Unknown Member"}
			}
			// Format action with type, date, and member
			buff.WriteString(fmt.Sprintf("**%s** (%s): %s - %s\n", action.Type, action.Date.Format("2006-01-02 15:04:05"), action.MemberCreator.FullName, action.Data.Text))
		}
		// Create markdown file for card history
		err := os.WriteFile(historyFileName, buff.Bytes(), SecureFileMode)
		if err != nil {
			logger("CRITICAL - Unable to write buffer to file for "+historyFileName+" Error: "+err.Error(), "err", true, true, config)
			errorWarnOnCompletion = true
			return err
		}
		logger("Created history markdown file: "+historyFileName, "info", true, true, config)
	} else {
		logger("No history found for card "+card.Name, "warn", true, true, config)
		// Create an empty history markdown file if no history found
		_ = os.WriteFile(historyFileName, nil, SecureFileMode)
	}
	return nil
}

/*
processCardDates creates markdown files for card due and start dates
*/
func processCardDates(card *trello.Card, cardPath string, config Config, dueFileName *string) error {
	// Save Card Due Date
	if card.Due != nil {
		if card.DueComplete {
			*dueFileName = filepath.Join(cardPath, "CardDueDate (Completed).md")
		} else {
			*dueFileName = filepath.Join(cardPath, "CardDueDate.md")
		}
		err := os.WriteFile(*dueFileName, []byte(card.Due.Format("2006-01-02 15:04:05")), SecureFileMode)
		if err != nil {
			logger("CRITICAL - Unable to write buffer to file for "+*dueFileName+" Error: "+err.Error(), "err", true, true, config)
			errorWarnOnCompletion = true
			return err
		}
		logger("Created due date markdown file: "+*dueFileName, "info", true, true, config)
	} else {
		logger("No due date found for card "+card.Name, "warn", true, true, config)
		// Create an empty due date markdown file if no due date found
		*dueFileName = filepath.Join(cardPath, "CardDueDate.md")
		_ = os.WriteFile(*dueFileName, nil, SecureFileMode)
	}

	// Save Card Start Date
	if card.Start != nil {
		startFileName := filepath.Join(cardPath, "CardStartDate.md")
		err := os.WriteFile(startFileName, []byte(card.Start.Format("2006-01-02 15:04:05")), SecureFileMode)
		if err != nil {
			logger("CRITICAL - Unable to write buffer to file for "+startFileName+" Error: "+err.Error(), "err", true, true, config)
			errorWarnOnCompletion = true
			return err
		}
		logger("Created start date markdown file: "+startFileName, "info", true, true, config)
	} else {
		logger("No start date found for card "+card.Name, "warn", true, true, config)
		// Create an empty start date markdown file if no start date found
		startFileName := filepath.Join(cardPath, "CardStartDate.md")
		_ = os.WriteFile(startFileName, nil, SecureFileMode)
	}
	return nil
}

/*
processCardCover saves card cover information (color or image reference)
*/
func processCardCover(card *trello.Card, cardPath string, config Config) error {
	if card.Cover == nil {
		logger("No cover set on card "+card.Name, "info", true, true, config)
	} else if card.Cover.Color != "" {
		colorFile := filepath.Join(cardPath, "CardCoverColor.md")
		if err := os.WriteFile(colorFile, []byte(card.Cover.Color), SecureFileMode); err != nil {
			logger("Error writing cover color for "+card.Name+": "+err.Error(), "err", true, false, config)
			return err
		}
	} else {
		logger("Cover is an image, already downloaded in attachments for card "+card.Name, "info", true, true, config)
	}
	return nil
}

/*
createListCache fetches all lists for the board once to avoid repeated API calls
*/
func createListCache(board *trello.Board, config Config) (map[string]*trello.List, error) {
	logger("Caching board lists for performance", "info", true, true, config)

	lists, err := board.GetLists(trello.Defaults())
	if err != nil {
		return nil, fmt.Errorf("failed to get board lists: %w", err)
	}

	listCache := make(map[string]*trello.List)
	for _, list := range lists {
		if list != nil {
			listCache[list.ID] = list
		}
	}

	logger(fmt.Sprintf("Cached %d lists for board %s", len(listCache), board.Name), "info", true, true, config)
	return listCache, nil
}

/*
getComprehensiveCardData fetches all card data in fewer API calls
*/
func getComprehensiveCardData(cardID string, client *trello.Client) (*trello.Card, error) {
	// Get card with all related data in one call
	args := trello.Arguments{
		"attachments":     "true",
		"actions":         "all",
		"actions_limit":   "1000",
		"members":         "true",
		"labels":          "all",
		"checklists":      "all",
		"checkItemStates": "true",
	}

	cardData, err := client.GetCard(cardID, args)
	if err != nil {
		return nil, fmt.Errorf("failed to get comprehensive card data for %s: %w", cardID, err)
	}

	return cardData, nil
}

/*
processCardsConcurrently manages concurrent processing of cards using a worker pool
*/
func processCardsConcurrently(cards []*trello.Card, board *trello.Board, boardPath string, config Config, client *trello.Client) {
	//  Cache all lists once instead of fetching per card
	listCache, err := createListCache(board, config)
	if err != nil {
		logger("Error caching board lists: "+err.Error(), "err", true, false, config)
		// Fallback to individual list calls
		listCache = make(map[string]*trello.List)
	}
	numCards := len(cards)
	if numCards == 0 {
		return
	}

	// Create channels for work distribution and result collection
	jobs := make(chan CardProcessingJob, numCards)
	results := make(chan error, numCards)

	// Progress tracking
	var processed int64

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < MaxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			processCardWorker(jobs, results, &processed)
		}()
	}

	// Send work to workers
	go func() {
		defer close(jobs)
		for i, card := range cards {
			jobs <- CardProcessingJob{
				card:      card,
				board:     board,
				boardPath: boardPath,
				config:    config,
				client:    client,
				listCache: listCache,
				index:     i,
				total:     numCards,
			}
		}
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Process results and handle errors
	errorCount := 0
	for i := 0; i < numCards; i++ {
		if err := <-results; err != nil {
			errorCount++
			logger("Card processing error: "+err.Error(), "err", true, false, config)
		}
	}

	if errorCount > 0 {
		logger(fmt.Sprintf("Completed with %d errors out of %d cards", errorCount, numCards), "warn", true, false, config)
		errorWarnOnCompletion = true
	}
}

/*
	Doing our own thing, as Trello Go Client doesn't support cardRole in card JSON as of 6/16/2025
*/
// Embed trello.Card and add CardRole
type CardWithRole struct {
	trello.Card
	CardRole *string `json:"cardRole,omitempty"`
}

// isLinkCard - is card a link
func isLinkCard(client *trello.Client, cardID string) (bool, error) {
	var cwr CardWithRole

	// Fetch just the fields we care about
	args := trello.Arguments{
		"fields": "name,cardRole",
	}
	if err := client.Get(fmt.Sprintf("cards/%s", cardID), args, &cwr); err != nil {
		return false, err
	}

	// cardRole will be nil (or empty) for normal cards
	if cwr.CardRole != nil && *cwr.CardRole == "link" {
		return true, nil
	}
	return false, nil
}

/*
dumpABoard - Process the board data and dump to specified directory structure

	assumes board ID is valid and exists
	assumes client is already authenticated with valid API key and token
*/
func dumpABoard(config Config, board *trello.Board, client *trello.Client) {

	var (
		cards     []*trello.Card
		err       error
		boardPath string
	)

	/*
		Build File System Structure
	*/
	// Create main directory
	dirCreate(config.ARGS.StoragePath)
	// Create directory in path named by board name
	boardPath = SanitizePathName(board.Name)
	dirCreate(config.ARGS.StoragePath + "/" + boardPath)

	// Stash in master slice for reference later
	boardTracker = append(boardTracker, board.Name+" ("+board.ID+")")

	/*
		Board Level Data
	*/
	// Save board background image if exists
	if board.Prefs.BackgroundImage != "" {
		url := board.Prefs.BackgroundImage
		localFilePath := filepath.Join(config.ARGS.StoragePath, boardPath, "BoardBackground-")
		err := downLoadFile(url, localFilePath)
		if err != nil {
			logger("Error: Unable to download background image for board "+board.Name+": "+err.Error(), "err", true, false, config)
		}
	} else {
		logger("No background image found for board"+board.Name, "info", true, true, config)
	}

	/*
		Create markdown list of labels and their names/colors
	*/

	logger("Grabbing labels for board and saving as Markdown BoardLabels.md", "info", true, true, config)

	labels, err := board.GetLabels(trello.Defaults())
	if err != nil {
		logger("Error: Unable to get label data for board ID "+board.ID+" ("+board.Name+")", "err", true, false, config)
	} else {

		buf := prettyPrintLabels(labels, true)

		// Write buffer content to a file
		labelFileName := filepath.Join(config.ARGS.StoragePath, boardPath, "BoardLabels.md")
		err := os.WriteFile(labelFileName, buf.Bytes(), SecureFileMode)
		if err != nil {
			logger("CRITICAL - Unable to write buffer to file for "+labelFileName+" Error: "+err.Error(), "err", true, true, config)
			errorWarnOnCompletion = true

			return
		}
	}

	/*
		Get Board Members
		- Create markdown file for board members
		- Include member name and ID
	*/
	logger("Grabbing members for board: "+board.Name, "info", true, true, config)

	members, err := board.GetMembers()
	if err != nil {
		logger("Error: Unable to get members for board ID "+board.ID, "err", true, true, config)
	} else {
		memberBuf := getBuffer()
		defer putBuffer(memberBuf)

		for _, member := range members {
			if member == nil {
				continue
			}
			memberBuf.WriteString(fmt.Sprintf("**%s** (%s)\n", member.FullName, member.ID))
		}
		// Write buffer content to a file
		memberFileName := filepath.Join(config.ARGS.StoragePath, boardPath, "BoardMembers.md")
		err := os.WriteFile(memberFileName, memberBuf.Bytes(), SecureFileMode)
		if err != nil {
			logger("CRITICAL - Unable to write buffer to file for "+memberFileName+" Error: "+err.Error(), "err", true, true, config)
			errorWarnOnCompletion = true

			return
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
		logger("Searching for only cards with label ID: "+config.ARGS.LabelID, "info", true, false, config)
		query := fmt.Sprintf("board:%s label:\"%s\" is:open", board.ID, config.ARGS.LabelID)
		logger("Querying Trello API with: "+sanitizeURLForLogging(query), "info", true, true, config)
		cards, err = client.SearchCards(query, trello.Defaults())
		if err != nil {
			handleProcessingError(
				newProcessingError("search cards by label", fmt.Sprintf("board %s, label %s", board.Name, config.ARGS.LabelID), ErrorSeverityCritical, err),
				config)
			return
		}
	} else {
		// If no specific label ID is provided, get all cards based on the -a flag
		if config.ARGS.Archived {
			cards, err = board.GetCards(trello.Arguments{"filter": "all"})
		} else {
			cards, err = board.GetCards(trello.Arguments{"filter": "open"})
		}
		if err != nil {
			handleProcessingError(
				newProcessingError("get board cards", fmt.Sprintf("board %s", board.Name), ErrorSeverityCritical, err),
				config)
			return
		}
	}

	// If no cards found, return with message
	if len(cards) == 0 {
		logger("CRITICAL - No cards found for board "+board.Name, "warn", true, false, config)
		errorWarnOnCompletion = true

		return
	} else {
		if len(cards) > 1 {
			logger("Found "+strconv.Itoa(len(cards))+" cards to process.\nPlease wait...\n", "info", true, false, config)
		} else {
			logger("Found "+strconv.Itoa(len(cards))+" card to processs.\nPlease wait...\n", "info", true, false, config)
		}
	}

	if !ListLoud && !config.ARGS.SuperQuiet {
		fmt.Println() // blank line to make counter output cleaner
	}

	// Process cards concurrently for better performance
	processCardsConcurrently(cards, board, boardPath, config, client)

	if !ListLoud && !config.ARGS.SuperQuiet {
		fmt.Println() // New line after running counter
	}
}
