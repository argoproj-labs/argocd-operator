#!/bin/bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

if ! cd "$SCRIPT_DIR"; then
    echo "Error: Failed to change directory to $SCRIPT_DIR" >&2
    exit 1
fi

# Run the upgrade code
go run .
