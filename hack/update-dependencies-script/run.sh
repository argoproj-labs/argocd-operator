#!/bin/bash
# This script will run all possible dependency upgrades
# These are all of the subdirectories that include a run.sh script

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)

if ! cd "$SCRIPT_DIR"; then
  echo "Error: Failed to change directory to $SCRIPT_DIR" >&2
  exit 1
fi

DIRECTORIES=(*)
for dir in "${DIRECTORIES[@]}"; do
  RUN_PATH="${dir}/run.sh"
  if [ -f "$RUN_PATH" ]; then
    echo "Updating dependencies for $dir..."
    if ! cd "$dir"; then
      echo "Error: Failed to change directory to $dir" >&2
      exit 1
    fi

    if ! bash ./run.sh; then
      echo "Error: Failed to update dependencies for $dir" >&2
      exit 1
    fi
    cd ../
  fi

done
