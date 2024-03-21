#!/bin/bash

# Define the diff file
DIFF_FILE="./etcd-diff.diff"

# Define the base path for etcd imports
ETCD_BASE_PATH="go.etcd.io/etcd"

# Extract changes from the diff file
grep -E "^\+[[:space:]]*\"$ETCD_BASE_PATH" $DIFF_FILE | awk -F '"' '{print $2}' | while read -r NEW_IMPORT; do
    OLD_IMPORT=$(grep -B 1 "$NEW_IMPORT" $DIFF_FILE | grep -E "^\-[[:space:]]*\"$ETCD_BASE_PATH" | awk -F '"' '{print $2}')
    if [ ! -z "$OLD_IMPORT" ]; then
        echo "Import changed from $OLD_IMPORT to $NEW_IMPORT"
    fi
done

