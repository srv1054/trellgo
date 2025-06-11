package main

import (
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
	StoragePath      string
	BoardID          string
	LabelID          string
}

type ENV struct {
	TRELLOAPIKEY string
	TRELLOAPITOK string
	TRELLOAPIURL string
}

// getCLIArgs - Get CLI arguments and flags
func getCLIArgs() (config ARGS) {

	var (
		// CLI Flags
		Archived         = flag.Bool("a", false, "Include archived cards in dump")
		BoardID          = flag.String("b", "", "Trello board to dump Unique Identifier")
		ListTotalCards   = flag.Bool("count", false, "List total number of cards in the board")
		LabelID          = flag.String("l", "", "Only include cards with this label ID")
		ListLabelIDs     = flag.Bool("labels", false, "Retrieve boards list of Label IDs")
		StoragePath      = flag.String("s", "", "Root Level path to store board information")
		SeparateArchived = flag.Bool("split", false, "Separate archived cards into their own directory")
		ver              = flag.Bool("v", false, "Version Check")
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
	config.SeparateArchived = *SeparateArchived

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

// printHelp - prints help menu when -h is used on CLI
func printHelp(version string) {
	fmt.Printf("\ttrellgo v%s by srv1054 (github.com/srv1054/trellgo)\n", version)
	fmt.Println()
	fmt.Println("Usage: ./trellgo [options]")
	fmt.Println("Options:")
	fmt.Printf("  -a\t\tInclude archived cards in dump\n")
	fmt.Printf("  -b\t\tTrello board to dump BoardID (REQUIRED)\n")
	fmt.Printf("  -l\t\tOnly include cards with this label ID\n")
	fmt.Printf("  -labels\tRetrieve boards list of Label IDs\n")
	fmt.Printf("  -count\tList total number of cards in the board\n")
	fmt.Printf("  -s\t\tRoot Level path to store board information (REQUIRED)\n")
	fmt.Printf("  -split\tSeparate archived cards into their own directory (instead of mixed in and labeled with -ARCHIVED\n")
	fmt.Printf("  -v\t\tPrints version and exits\n")
	fmt.Println()
	fmt.Printf("Example: trellgo -b c52d11s -l ff3sg135 -s '/path/to/here'\n")
	fmt.Printf("Example: trellgo -labels")
	fmt.Println()
	os.Exit(0)
}

// dirCreate - Create main directory if it doesn't exist
func dirCreate(storagePath string) {
	// check if passed directory exists if not create it
	if _, err := os.Stat(storagePath); os.IsNotExist(err) {
		fmt.Println("Creating requested directory:", storagePath)
		err := os.MkdirAll(storagePath, os.ModePerm)
		if err != nil {
			fmt.Println("Error: Unable to create requested directory:", storagePath)
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		fmt.Println("Requested directory already exists:", storagePath)
	}
}

// prettyPrintLabels - Print out the labels output in a pretty table
func prettyPrintLabels(labels []*trello.Label, markdown bool) bytes.Buffer {

	var (
		t   = table.NewWriter()
		buf bytes.Buffer
	)

	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Label Name", "Label Color", "Label UID"})

	for _, label := range labels {
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

// SanitizePath - Sanitize the path for file system before creating directories.  Returns sanitized string
func SanitizePathName(name string) string {
	// Define allowed characters (letters, numbers, underscores, dashes, and dots)
	re := regexp.MustCompile(`[^a-zA-Z0-9 ._-]`)

	// Replace disallowed characters with underscores
	sanitized := re.ReplaceAllString(name, "-")

	// Trim leading and trailing dots or underscores to avoid hidden files or empty names
	sanitized = strings.Trim(sanitized, "._-")

	// Ensure it is not empty
	if sanitized == "" {
		fmt.Printf("\n\nRequested path name %v is empty after sanitization\nExit!\n\n", name)
		os.Exit(1)
	}

	// Limit length (optional, e.g., 255 characters)
	if len(sanitized) > 255 {
		sanitized = sanitized[:255]
	}

	return sanitized
}

// downLoadFile - Download a remote file to the local drive for certain types of attachments, like board background images
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

	fmt.Println("Downloading file named", fileName, "from URL:", fileURL, "to local path:", filePath)

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

	fmt.Println("Downloaded:", filePath)
	return nil
}

// downloadFileAuthHeader - download file from URL to local file system when trello requires API authentication, likfe files attached to cards (PDF, etc)
func downloadFileAuthHeader(fileURL string, localFilePath string, apiKey string, apiToken string) error {

	u, err := url.Parse(fileURL)
	if err != nil {
		log.Fatalf("invalid URL: %v", err)
	}

	// Extract filename from the URL
	fileName := path.Base(u.Path)
	fullFile := localFilePath + fileName

	fmt.Println("Downloading file named", fileName, "from URL:", fileURL, "to local path:", fullFile)

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
	out, err := os.Create(fullFile)
	if err != nil {
		return err
	}
	defer out.Close()

	// Copy the response body to the file
	_, err = io.Copy(out, resp.Body)
	return err
}
