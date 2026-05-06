#!/bin/bash
FILE_PATH=$(jq -r '.tool_input.file_path' < /dev/stdin)
if [[ "$FILE_PATH" != *.go ]]; then exit 0; fi
if [[ ! -f "$FILE_PATH" ]]; then exit 0; fi
goimports -w "$FILE_PATH" 2>&1
exit 0
