# trellgo
This service slurps down trello boards and cards and converts them into file system structures, with attachments, Markdown documents, etc  

Preserving Trello data in a non-cloud format.  You know in case they go away and I want me freakin data in a consumable format that's not a CSV, XLSX, or JSON file.

See `trellgo -h` for available parameters and help.  

This requires a Trello API Token and a Key, so get those first: https://developer.atlassian.com/cloud/trello/guides/rest-api/api-introduction/

### File Structure
You are required to specify a top level path for where things will be saved, using `-s`.   
Underneath that path the file structure will look like this:  

```
/Board Name
  /List Name
    /Card Name
      /Attachments
        Downloaded attachment files (pdf, jpg, etc)
      /Checklists
        Markdown text files
      Markdown Text files with specific card data in them such as users, descriptions, history, etc
Board Background image file
Markdown Text file of Board data (Labels, Members, etc)
```

### Additional Data retreival
You can use the `-label` parameter and get a prettied dump of all the Labels available on a board, in case you want to dump the board based on a specific label.  
You can use the `-count` parameter and get a prettified card count of Open Cards, Visible Cards, and Archived (closed) Cards

### Extra logging info
Right now minimal info is dumped to the console when you run the binary, by design, however if you want gobs of information to see what's going on, add `-loud` to the CLI paramemter list.  

Be nice feature to add some sort of logging so you can get a quite console and get all the loud stuff into a log behind the scenes.   I've added that to the `feature.md` file.
