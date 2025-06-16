package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/adlio/trello"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/joho/godotenv"
)

type ARGS struct {
	Archived         bool
	ListLabelIDs     bool
	ListTotalCards   bool
	SeparateArchived bool
	SuperQuiet       bool
	LoggingEnabled   bool
	StoragePath      string
	LabelID          string
	LogFile          string
}

type ENV struct {
	TRELLOAPIKEY string
	TRELLOAPITOK string
	TRELLOAPIURL string
}

/*
getCLIArgs

	Get CLI arguments and flags
*/
func getCLIArgs() (config ARGS, boards []string) {

	var (
		// CLI Flags
		Archived         = flag.Bool("a", false, "")
		BoardID          = flag.String("b", "", "")
		ListTotalCards   = flag.Bool("count", false, "")
		LabelID          = flag.String("l", "", "")
		ListLabelIDs     = flag.Bool("labels", false, "")
		LogFile          = flag.String("logs", "", "")
		Loud             = flag.Bool("loud", false, "")
		QQ               = flag.Bool("qq", false, "")
		StoragePath      = flag.String("s", "", "n")
		SeparateArchived = flag.Bool("split", false, "")
		ver              = flag.Bool("v", false, "")
	)

	// Handle -h help
	flag.Usage = func() { printHelp(version) }

	// Parse CLI flags
	flag.Parse()

	// Set config values
	config.Archived = *Archived
	config.LabelID = *LabelID
	config.ListLabelIDs = *ListLabelIDs
	config.ListTotalCards = *ListTotalCards
	config.StoragePath = *StoragePath
	config.SeparateArchived = *SeparateArchived
	config.SuperQuiet = *QQ
	config.LogFile = *LogFile

	ListLoud = *Loud

	// Handle -v version
	if *ver {
		fmt.Printf("trellgo v%s\n", version)
		os.Exit(0)
	}

	// Check if we need to use STDIN (Pipe) or -b for BoardIDs
	boards, err := getBoardIDs(*BoardID, os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		flag.Usage()
		os.Exit(1)
	}

	// Check for required flag of Storage Path if not using -labels or -count
	if !*ListLabelIDs && !*ListTotalCards && *StoragePath == "" {
		fmt.Println("Error: No Storage Path provided. REQUIRED")
		printHelp(version)
		os.Exit(1)
	}

	// Searching on a specific Label will not allow search of archives, need to inform user
	if *LabelID != "" && *Archived {
		fmt.Println("Error: Cannot use -l flag with -a flag. Use -l without -a to filter by label ID")
		printHelp(version)
		os.Exit(1)
	}

	return config, boards
}

/*
getOSENV

	Get Trello API Key from OS Environment
*/
func getOSENV() (config ENV) {

	// Load vars in dotenv file if it exists (preferred method)
	if _, err := os.Stat(".env"); err == nil {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file, it exists but is not readable")
		}
	} else {
		fmt.Println("No .env file found, using OS Environment variables")
	}

	config.TRELLOAPIKEY = os.Getenv("TRELLGO_APIKEY")
	config.TRELLOAPITOK = os.Getenv("TRELLGO_APITOK")
	config.TRELLOAPIURL = os.Getenv("TRELLGO_APIURL")

	if config.TRELLOAPIKEY == "" || config.TRELLOAPITOK == "" {
		fmt.Println("Error: No Trello API Key or Token provided in OS Environment")
		fmt.Println("Exiting...")
		os.Exit(1)
	}

	return config
}

/*
getBoardIDs

	Get Board IDs from CLI flag or stdin
	If the -b flag is used, it will return that ID.
*/
func getBoardIDs(boardFlag string, stdin io.Reader) ([]string, error) {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}

	var ids []string
	// if stdin is piped (not a terminal), read lines
	if fi.Mode()&os.ModeCharDevice == 0 {
		scanner := bufio.NewScanner(stdin)
		for scanner.Scan() {
			if line := scanner.Text(); line != "" {
				ids = append(ids, line)
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
	}

	// if nothing came in on stdin, use the -b flag
	if len(ids) == 0 {
		if boardFlag == "" {
			return nil, fmt.Errorf("no board IDs provided (pipe them in or use -b)")
		}
		ids = append(ids, boardFlag)
	}

	return ids, nil
}

/*
printHelp

	Prints help menu when -h is used on CLI
*/
func printHelp(version string) {
	fmt.Printf("\ttrellgo v%s by srv1054 (github.com/srv1054/trellgo)\n", version)
	fmt.Println()
	fmt.Println("Usage: ./trellgo [options]")
	fmt.Println("Options:")
	fmt.Printf("  -a\t\tInclude archived cards in dump\n")
	fmt.Printf("  -b\t\tTrello board to dump BoardID or PIPE (|) IDs in one per line. (REQUIRED if not piping from STDIN)\n")
	fmt.Printf("  -count\tList total number of cards in the board\n")
	fmt.Printf("  -l\t\tOnly include cards with this label NAME (Does not work with -a flag. Requires NAME of label \"in quotes\", not ID)\n")
	fmt.Printf("  -labels\tRetrieve boards list of Label IDs\n")
	fmt.Printf("  -loud\t\tEnable more verbose output\n")
	fmt.Printf("  -logs \"file\"\tSpecifies a log file to send all output. Off by default, if enabled, its not effected by -loud or -qq parameters.\n")
	fmt.Printf("  -qq\t\tSuppress ALL console output.  Super Quiet mode.  Does not effect logging, just console.  Does not apply to -labels or -count\n")
	fmt.Printf("  -s\t\tRoot Level path to store board information (REQUIRED)\n")
	fmt.Printf("  -split\tSeparate archived cards into their own directory (instead of mixed in and labeled with -ARCHIVED)\n")
	fmt.Printf("  -v\t\tPrints version and exits\n")
	fmt.Println()
	fmt.Println("Console output is minimal by default, with high level messages.  Use -loud to enable more verbose output.  Errors always print to console.")
	fmt.Println()
	fmt.Printf("Example: trellgo -b c52d11s -l ff3sg135 -s '/path/to/here'\n")
	fmt.Printf("Example: trellgo -b c52d11s -a -split -s '/path/to/here'\n")
	fmt.Printf("Example: trellgo -b c52d11s -s '/path/to/here' -logs '/path/file.log'\n")
	fmt.Printf("Example: trellgo -b t532aad -labels\n")
	fmt.Printf("Example: trellgo -b 5f3g1a2 -count\n")
	fmt.Println()
	os.Exit(0)
}

/*
dirCreate

	Create a directory if it doesn't exist
*/
func dirCreate(storagePath string) {
	// check if passed directory exists if not create it
	if _, err := os.Stat(storagePath); os.IsNotExist(err) {

		logger("Creating requested directory:"+storagePath, "info", true, true, config)

		err := os.MkdirAll(storagePath, os.ModePerm)
		if err != nil {
			logger("Error: Unable to create requested directory "+storagePath+": "+err.Error(), "err", true, false, config)
			os.Exit(1)
		}
	} else {
		logger("Requested directory already exists: "+storagePath, "info", true, true, config)
	}
}

/*
prettyPrintLabels

	Print out the labels output in a pretty table
*/
func prettyPrintLabels(labels []*trello.Label, markdown bool) bytes.Buffer {

	var (
		t   = table.NewWriter()
		buf bytes.Buffer
	)

	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Label Name", "Label Color", "Label UID"})

	for _, label := range labels {
		if label == nil {
			continue
		}
		if label.Color == "" {
			label.Color = "No Color"
		}
		if label.Name == "" {
			label.Name = "No Name"
		}
		t.AppendRow([]interface{}{label.Name, label.Color, label.ID})
		t.AppendSeparator()
	}

	// Set style
	t.SetStyle(table.StyleLight)
	t.Style().Color.Header = text.Colors{text.FgHiGreen, text.Bold}

	// Render a markdown table
	if markdown {
		// Put in a buffer instead of console
		t.SetOutputMirror(&buf)

		// Render the table to the buffer
		t.RenderMarkdown()

		// Return the buffage
		return buf

	} else {
		// Render normal table to console
		t.Render()
	}

	fmt.Println()

	// Return empty buffer
	return buf
}

/*
SanitizePath

	Sanitize the path for file system before creating directories.
	Returns sanitized string that can be used as a directory name.  Windows and Linux safe.
*/
func SanitizePathName(name string) string {

	var cleaned string

	// Remove characters illegal on Windows and Linux filesystems
	re := regexp.MustCompile(`[<>:"/\\|?*]`)
	cleaned = re.ReplaceAllString(name, "-")

	// Trim leading and trailing chars to avoid hidden files or empty names
	cleaned = strings.Trim(cleaned, " ._-")

	// Ensure it is not empty
	if cleaned == "" {
		logger("Requested path name "+name+" is empty after sanitization", "error", true, false, config)
		cleaned = fmt.Sprintf("Board-Was-Illegal-Characters-%s", time.Now().Format("20060102-150405"))
		logger("Using fallback name: "+cleaned, "info", true, false, config)
	}

	// Limit length to 240 characters
	// This leaves 15 chars for outside func appends without cause panic "filename to long"
	if len(cleaned) > 240 {
		cleaned = cleaned[:240]
	}

	return cleaned
}

/*
downLoadFile

	Download a remote file to the local drive for certain types of attachments, like board background images
*/
func downLoadFile(fileURL string, localFilePath string) error {

	var (
		filePath string
	)

	u, err := url.Parse(fileURL)
	if err != nil {
		log.Fatalf("invalid URL: %v", err)
	}

	// Extract filename from the URL
	fileName := path.Base(u.Path)
	if fileName == "" {
		filePath = localFilePath + "UnknownFile"
	} else {
		filePath = localFilePath + fileName
	}

	logger("Downloading file named "+fileName+" from URL: "+fileURL+" to local path: "+filePath, "info", true, true, config)

	// Create the file
	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(fileURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	logger("Downloaded: "+filePath, "info", true, true, config)

	return nil
}

/*
downloadFileAuthHeader

	Download file from URL to local file system when trello requires API authentication, likfe files attached to cards (PDF, etc)
*/
func downloadFileAuthHeader(fileURL string, localFilePath string, apiKey string, apiToken string) error {

	logger("Downloading file from URL: "+fileURL+" to local path: "+localFilePath, "info", true, true, config)

	// Create a new HTTP request with Authorization header
	req, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		return err
	}

	// Add Authorization token
	req.Header.Set("Authorization", fmt.Sprintf("OAuth oauth_consumer_key=\"%s\", oauth_token=\"%s\"", apiKey, apiToken))

	// Execute the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check if the response is OK
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: %s (status: %d)", fileURL, resp.StatusCode)
	}

	// Create the file
	out, err := os.Create(localFilePath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Copy the response body to the file
	_, err = io.Copy(out, resp.Body)
	return err
}
