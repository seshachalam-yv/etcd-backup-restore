package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"go.etcd.io/etcd/clientv3"
)

func main() {
	// Configure client
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to connect to ETCD: %v", err)
	}
	defer cli.Close()

	// Start snapshot
	snapshotReader, err := cli.Maintenance.Snapshot(context.Background())
	if err != nil {
		log.Fatalf("Failed to take snapshot: %v", err)
	}
	defer snapshotReader.Close()

	// Buffer to hold the snapshot data
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, snapshotReader); err != nil {
		log.Fatalf("Failed to read snapshot data: %v", err)
	}

	// Extract hash and verify
	if err := verifySnapshotHash(buf); err != nil {
		log.Fatalf("Snapshot verification failed: %v", err)
	}

	// Save verified snapshot to file
	if err := saveSnapshotToFile("etcd-snapshot.db", buf); err != nil {
		log.Fatalf("Failed to save verified snapshot: %v", err)
	}
	log.Println("Snapshot verification succeeded and saved!")
}

func verifySnapshotHash(buf *bytes.Buffer) error {
	data := buf.Bytes()
	if len(data) < sha256.Size {
		return fmt.Errorf("data length less than hash size")
	}

	// Split data and hash
	actualData := data[:len(data)-sha256.Size]
	storedHash := data[len(data)-sha256.Size:]

	// Compute hash of the data
	hasher := sha256.New()
	if _, err := hasher.Write(actualData); err != nil {
		return fmt.Errorf("error hashing data: %v", err)
	}
	computedHash := hasher.Sum(nil)

	// Pretty print stored and computed hashes
	fmt.Printf("Stored hash: %x\n", storedHash)
	fmt.Printf("Computed hash: %x\n", computedHash)

	if !equalHashes(storedHash, computedHash) {
		return fmt.Errorf("hash mismatch: expected %x, got %x", storedHash, computedHash)
	}
	return nil
}

func saveSnapshotToFile(filename string, buf *bytes.Buffer) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	if _, err := buf.WriteTo(file); err != nil {
		return fmt.Errorf("failed to write to file: %v", err)
	}
	return nil
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
