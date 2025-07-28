# Trellgo Program Overview

This program, `trellgo`, is a command-line tool written in Go that exports Trello board data into a structured filesystem format. Its main purpose is to preserve Trello data locally, including boards, lists, cards, attachments, checklists, comments, users, labels, and history, in Markdown and file formats.

---

## How It Works

### 1. Configuration and Startup
- Reads CLI arguments to determine what to export and where to save it.
- Loads Trello API credentials from a `.env` file or environment variables.
- Logging can be enabled to a file via the `-logs` parameter.

### 2. Board Selection
- Boards can be specified via the `-b` flag or piped in via STDIN.
- The main loop processes each board ID.

### 3. Data Retrieval and Export
For each board, the program:
- Creates a directory structure based on board, list, and card names.
- Downloads the board background image if present.
- Exports board labels and members to Markdown files.
- Retrieves cards, optionally filtering by label or including archived cards.
- For each card:
  - Creates a directory (or Markdown file for link cards).
  - Exports card description, attachments (downloads files and lists URLs), checklists, comments, users, labels, history, due/start dates, and cover color/image as Markdown or files.
  - Handles archived cards by either labeling or moving them to an `/ARCHIVED` directory.

### 4. Special Features
- Can list all labels on a board (`-labels`).
- Can count cards by status (`-count`).
- Supports verbose output (`-loud`) and super-quiet mode (`-qq`).
- Handles Trello's new `cardRole` field for link cards.

### 5. Error Handling and Logging
- Errors are logged and reported, with critical errors flagged for user attention.
- All output can be sent to a log file regardless of console verbosity.

---

## Example Usage

```sh
trellgo -b <BoardID> -s "C:/path/to/save"
trellgo -b <BoardID> -a -split -s "C:/path/to/save"
trellgo -b <BoardID> -labels
trellgo -b <BoardID> -count
```

See `README.md` for more details and examples.

---

**Summary:**  
`trellgo` is a utility for backing up Trello boards to your local filesystem, organizing all board data into readable Markdown and file formats, with flexible options for filtering, logging, and