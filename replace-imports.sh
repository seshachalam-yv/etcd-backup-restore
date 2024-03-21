#!/bin/bash
set -xe
# Used by current backup-restore
# grep -R --include="*.go" --exclude-dir="vendor" 'go.etcd.io/etcd' . | awk '{print $2}' | sort | uniq
# "go.etcd.io/etcd/clientv3"
# "go.etcd.io/etcd/embed"
# "go.etcd.io/etcd/etcdserver"
# "go.etcd.io/etcd/etcdserver/api/membership"
# "go.etcd.io/etcd/etcdserver/api/snap"
# "go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes"
# "go.etcd.io/etcd/etcdserver/etcdserverpb"
# "go.etcd.io/etcd/lease"
# "go.etcd.io/etcd/mvcc"
# "go.etcd.io/etcd/mvcc/backend"
# "go.etcd.io/etcd/mvcc/mvccpb"
# "go.etcd.io/etcd/pkg/traceutil"
# "go.etcd.io/etcd/pkg/transport"
# "go.etcd.io/etcd/pkg/types"
# "go.etcd.io/etcd/raft"
# "go.etcd.io/etcd/raft/raftpb"
# "go.etcd.io/etcd/wal"
# "go.etcd.io/etcd/wal/walpb"

# Define the mapping of old imports to new imports
declare -A import_map=(
    ["\"go.etcd.io/etcd/clientv3\""]='"go.etcd.io/etcd/client/v3"'
    ["\"go.etcd.io/etcd/embed\""]='"go.etcd.io/etcd/server/v3/embed"'
    ["\"go.etcd.io/etcd/etcdserver\""]='"go.etcd.io/etcd/server/v3/etcdserver"'
    ["\"go.etcd.io/etcd/etcdserver/api/membership\""]='"go.etcd.io/etcd/server/v3/etcdserver/api/membership"'
    ["\"go.etcd.io/etcd/etcdserver/api/snap\""]='"go.etcd.io/etcd/server/v3/etcdserver/api/snap"'
    ["\"go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes\""]='"go.etcd.io/etcd/server/v3/etcdserver/api/v3rpc/rpctypes"'
    ["\"go.etcd.io/etcd/etcdserver/etcdserverpb\""]='"go.etcd.io/etcd/server/v3/etcdserver/etcdserverpb"'
    ["\"go.etcd.io/etcd/lease\""]='"go.etcd.io/etcd/server/v3/lease"'
    ["\"go.etcd.io/etcd/mvcc\""]='"go.etcd.io/etcd/server/v3/mvcc"'
    ["\"go.etcd.io/etcd/mvcc/backend\""]='"go.etcd.io/etcd/server/v3/mvcc/backend"'
    ["\"go.etcd.io/etcd/mvcc/mvccpb\""]='"go.etcd.io/etcd/server/v3/mvcc/mvccpb"'
    ["\"go.etcd.io/etcd/pkg/traceutil\""]='"go.etcd.io/etcd/pkg/traceutil"'
    ["\"go.etcd.io/etcd/pkg/transport\""]='"go.etcd.io/etcd/server/v3/transport"'
    ["\"go.etcd.io/etcd/pkg/types\""]='"go.etcd.io/etcd/api/v3/etcdserverpb"'
    ["\"go.etcd.io/etcd/raft\""]='"go.etcd.io/etcd/raft/v3"'
    ["\"go.etcd.io/etcd/raft/raftpb\""]='"go.etcd.io/etcd/raft/v3/raftpb"'
    ["\"go.etcd.io/etcd/wal\""]='"go.etcd.io/etcd/server/v3/wal"'
    ["\"go.etcd.io/etcd/wal/walpb\""]='"go.etcd.io/etcd/server/v3/wal/walpb"'
)

# Find all Go files and replace the old imports with new imports
find . -type f -name "*.go" -not -path "./vendor/*" | while read -r file; do
    echo "Processing $file"
    for old_import in "${!import_map[@]}"; do
        new_import=${import_map[$old_import]}
        sed -i "s|${old_import}|${new_import}|g" "$file"
    done
done

echo "Import paths updated."
