// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

const (
	forceNewClusterMarkerFile = "force-new-cluster"
	boltDBTimeout             = 3 * time.Second
)

// isPostScaleDown detects whether etcd needs to start with force-new-cluster after a scale-down.
// It checks:
// 1. The data directory exists and has a valid member structure (WAL + snap/db)
// 2. The configured cluster size is 1 (single-node mode)
//
// When these conditions are met and the data directory existed from a prior multi-node run,
// force-new-cluster resets the raft cluster to a single-node cluster preserving all data.
// For an already single-node cluster, force-new-cluster is safe — it just resets raft state.
func isPostScaleDown(dataDir string, clusterSize int, logger *logrus.Entry) bool {
	if clusterSize != 1 {
		return false
	}

	if dataDir == "" {
		return false
	}

	// Check for explicit marker file first (most reliable, set by previous init cycle)
	markerPath := filepath.Join(filepath.Dir(dataDir), forceNewClusterMarkerFile)
	if _, err := os.Stat(markerPath); err == nil {
		logger.Info("Found force-new-cluster marker file, indicating post-scale-down state")
		return true
	}

	// Check data directory has the boltdb backend (meaning etcd previously ran)
	dbPath := filepath.Join(dataDir, "member", "snap", "db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return false
	}

	// If boltdb exists and cluster is configured with 1 member, check member count.
	// After MemberRemove, boltdb will have 1 member even though raft state may have >1 voters.
	// We need force-new-cluster to reset the raft state.
	memberCount := getMemberCountFromDB(dbPath, logger)
	if memberCount >= 1 {
		// Data directory has existing etcd data and cluster is now single-node.
		// Use force-new-cluster to ensure raft is properly reset to single-node.
		// This is safe even for already-single-node clusters (idempotent).
		logger.Infof("Post-scale-down recovery: boltdb has %d members, cluster configured for 1. Using force-new-cluster.", memberCount)
		return true
	}

	return false
}

// getMemberCountFromDB reads the number of members from etcd's boltdb members bucket.
func getMemberCountFromDB(dbPath string, logger *logrus.Entry) int {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return 0
	}

	db, err := bolt.Open(dbPath, 0400, &bolt.Options{Timeout: boltDBTimeout, ReadOnly: true})
	if err != nil {
		logger.Infof("Cannot open boltdb for member count check: %v", err)
		return 0
	}
	defer db.Close()

	var count int
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("members"))
		if b == nil {
			return nil
		}
		count = b.Stats().KeyN
		return nil
	})
	if err != nil {
		logger.Infof("Error reading members bucket: %v", err)
		return 0
	}

	return count
}

// writeForceNewClusterMarker writes a marker file that signals serveConfig to add force-new-cluster.
func writeForceNewClusterMarker(dataDir string, logger *logrus.Entry) error {
	markerPath := filepath.Join(filepath.Dir(dataDir), forceNewClusterMarkerFile)
	logger.Infof("Writing force-new-cluster marker at %s", markerPath)
	return os.WriteFile(markerPath, []byte("scale-down-detected"), 0600)
}

// removeForceNewClusterMarker removes the marker file after it has been consumed.
func removeForceNewClusterMarker(dataDir string, logger *logrus.Entry) {
	markerPath := filepath.Join(filepath.Dir(dataDir), forceNewClusterMarkerFile)
	if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
		logger.Warnf("Failed to remove force-new-cluster marker: %v", err)
	}
}
