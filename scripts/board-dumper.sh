#!/usr/bin/env bash
set -euo pipefail

# board-dumper.sh
# Dump and then Backup Trello Board Dumps v1.1 @srv1054
# For multiple trello boards.
# Leverages `trellgo` go application written by @srv1054 : https://github.com/srv1054/trellgo/

# CRON Example for weekly dump:  0 2 * * 1 /opt/trellgo/board-dumper.sh

TODAY=$(date +%m-%d-%Y)

# User settable vars
TRELLGOHOME="/opt/trellgo"
REMPATH="/opt/trellgo/backups" # Where to store remote gzips of boards
FILE="trello-board-dump-$TODAY.tar.gz"
FAC="local1"
BOARDLIST="/opt/trellgo/my.board.list" # List of Board UIDs one per line
LOCALARCHIVE="$TRELLGOHOME/archives" # Where to store local gzips of boaards

# cd to trellgo home to find all the things
cd $TRELLGOHOME

# source auth stuff
# Contains remote USER/HOST vars, assumes Keyless Auth on USER
. ./.authystuff

# Start
logger -p ${FAC}.info -t trellgo "Trellgo process complete"

# Dump Dynamic Boards
VERSION=`$TRELLGOHOME/trellgo -v`
logger -p ${FAC}.info -t trellgo "Starting trellgo v$VERSION. Doing trello board dump from $BOARDLIST"
cat $TRELLGOHOME/$BOARDLIST | $TRELLGOHOME/trellgo -a -qq -logs "$TRELLGOHOME/logs/$TODAY-live-dump.log" -s "/opt/trellgo/weekly"

# Tar and compress
logger -p ${FAC}.info -t trellgo "Starting trello board dump tar and compression: $TRELLGOHOME/$FILE"
tar -czf "$TRELLGOHOME/$FILE" -C "$TRELLGOHOME" weekly

# Send to remote destination
logger -p ${FAC}.info -t trellgo "Starting trello board scp to remote destination"
logger -p ${FAC}.info -t trellgo "scp -P 2273 $TRELLGOHOME/$FILE sending to $USER@$HOST:$REMPATH/"
scp -P 2273 $TRELLGOHOME/$FILE $USER@$HOST:$REMPATH/

# Cleanup Raw board dumps, keep zip files on both locations
logger -p ${FAC}.info -t trellgo "Cleaning up raw board dumps in $TRELLGOHOME/weekly/"
rm -fR $TRELLGOHOME/weekly/*

# Moves zips into local archive directory
logger -p ${FAC}.info -t trellgo "Moving $FILE to archive directory $LOCALARCHIVE/"
mv $TRELLGOHOME/$FILE $LOCALARCHIVE/$FILE

logger -p ${FAC}.info -t trellgo "Trellgo process complete"
logger -p ${FAC}.info -t trellgo "trellgo logs can be found at $TRELLGOHOME/logs/$TODAY-live-dump.log"

# Update slack that we did this, cause I'll forget.  Using slackcli binary @srv1054 - https://github.com/srv1054/slackcli
slackcli -c "server-messages" -cfg /etc/growly.json -m "*Completed* trello board dump for \`$TRELLGOHOME/$FILE\`. Logs: \`$TRELLGOHOME/logs/$TODAY-live-dump.log\`"

