package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"go.etcd.io/etcd/clientv3"
)

// BenchmarkSnapshot benchmarks the snapshot process
func BenchmarkSnapshot(b *testing.B) {
	for n := 0; n < b.N; n++ {
		// Configure client
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{"localhost:2379"},
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			b.Fatalf("Failed to connect to ETCD: %v", err)
		}
		defer cli.Close()

		// Start snapshot
		snapshotReader, err := cli.Maintenance.Snapshot(context.Background())
		if err != nil {
			b.Fatalf("Failed to take snapshot: %v", err)
		}
		defer snapshotReader.Close()

		// Generate dynamic file names based on iteration index
		snapshotFile := fmt.Sprintf("etcd-snapshot-%d.db", n)
		actualSnapshotFile := fmt.Sprintf("etcd-snapshot-actual-%d.db", n)

		// Create a file to write the snapshot data
		file, err := os.Create(snapshotFile)
		if err != nil {
			b.Fatalf("Failed to create snapshot file: %v", err)
		}
		defer file.Close()

		file2, err := os.Create(actualSnapshotFile)
		if err != nil {
			b.Fatalf("Failed to create snapshot file: %v", err)
		}
		defer file2.Close()

		// teeReader := io.TeeReader(snapshotReader, file2)

		// Write snapshot to file without computing hash at this point
		if _, err := io.Copy(file, snapshotReader); err != nil {
			b.Fatalf("Failed to write snapshot to file: %v", err)
		}

		// Now that the snapshot is fully written, read the file size
		fileStat, err := file.Stat()
		if err != nil {
			b.Fatalf("Failed to stat snapshot file: %v", err)
		}
		fileSize := fileStat.Size()

		// The snapshot must be larger than the hash size (32 bytes)
		if fileSize < sha256.Size {
			b.Fatalf("Snapshot file is too small to contain a valid hash")
		}

		// Seek to the last 32 bytes of the file to retrieve the stored hash
		if _, err := file.Seek(-sha256.Size, io.SeekEnd); err != nil {
			b.Fatalf("Failed to seek to stored hash in snapshot: %v", err)
		}

		// Read the stored hash from the file
		storedHash := make([]byte, sha256.Size)
		if _, err := file.Read(storedHash); err != nil {
			b.Fatalf("Failed to read stored hash from snapshot: %v", err)
		}

		// Truncate the file to remove the stored hash (last 32 bytes)
		if err := file.Truncate(fileSize - sha256.Size); err != nil {
			b.Fatalf("Failed to truncate snapshot file: %v", err)
		}

		// Rewind file to the start for computing the hash of the actual data
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			b.Fatalf("Failed to seek to start of snapshot file: %v", err)
		}

		// Now calculate the hash of the truncated file
		hasher := sha256.New()
		if _, err := io.Copy(hasher, file); err != nil {
			b.Fatalf("Failed to compute hash of snapshot file: %v", err)
		}
		computedHash := hasher.Sum(nil)

		// Verify hash (this is part of correctness)
		if !equalHashes(storedHash, computedHash) {
			b.Fatalf("Hash mismatch: expected %x, got %x", storedHash, computedHash)
		}
	}
}

// Helper function to check equality of two hashes
func equalHashes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
