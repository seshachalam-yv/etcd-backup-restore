// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package member

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

func TestWasMemberPreviouslyRemoved(t *testing.T) {
	tests := []struct {
		name           string
		memberName     string
		setupDB        func(t *testing.T, dbPath string)
		createDB       bool
		expectedResult bool
		expectedErr    bool
	}{
		{
			name:           "returns false when data directory does not exist",
			memberName:     "etcd-source-0",
			createDB:       false,
			expectedResult: false,
		},
		{
			name:       "returns false when no buckets exist",
			memberName: "etcd-source-0",
			createDB:   true,
			setupDB: func(t *testing.T, dbPath string) {
				// Create empty db - no buckets
				db, err := bolt.Open(dbPath, 0600, nil)
				if err != nil {
					t.Fatal(err)
				}
				db.Close()
			},
			expectedResult: false,
		},
		{
			name:       "returns false when member is in members bucket even with members_removed entries",
			memberName: "etcd-source-0",
			createDB:   true,
			setupDB: func(t *testing.T, dbPath string) {
				db, err := bolt.Open(dbPath, 0600, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				err = db.Update(func(tx *bolt.Tx) error {
					mb, err := tx.CreateBucket([]byte("members"))
					if err != nil {
						return err
					}
					data, _ := json.Marshal(map[string]any{
						"id":       1234,
						"name":     "etcd-source-0",
						"peerURLs": []string{"http://etcd-source-0:2380"},
					})
					if err := mb.Put([]byte("0000000000001234"), data); err != nil {
						return err
					}

					rb, err := tx.CreateBucket([]byte("members_removed"))
					if err != nil {
						return err
					}
					return rb.Put([]byte("0000000000005678"), []byte("removed"))
				})
				if err != nil {
					t.Fatal(err)
				}
			},
			expectedResult: false,
		},
		{
			name:       "returns true when member is absent from members but members_removed has entries",
			memberName: "etcd-source-0",
			createDB:   true,
			setupDB: func(t *testing.T, dbPath string) {
				db, err := bolt.Open(dbPath, 0600, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				err = db.Update(func(tx *bolt.Tx) error {
					mb, err := tx.CreateBucket([]byte("members"))
					if err != nil {
						return err
					}
					// Only other members in the bucket
					data1, _ := json.Marshal(map[string]any{
						"id":       5678,
						"name":     "etcd-source-1",
						"peerURLs": []string{"http://etcd-source-1:2380"},
					})
					if err := mb.Put([]byte("0000000000005678"), data1); err != nil {
						return err
					}
					data2, _ := json.Marshal(map[string]any{
						"id":       9012,
						"name":     "etcd-source-2",
						"peerURLs": []string{"http://etcd-source-2:2380"},
					})
					if err := mb.Put([]byte("0000000000009012"), data2); err != nil {
						return err
					}

					rb, err := tx.CreateBucket([]byte("members_removed"))
					if err != nil {
						return err
					}
					return rb.Put([]byte("0000000000001234"), []byte("removed"))
				})
				if err != nil {
					t.Fatal(err)
				}
			},
			expectedResult: true,
		},
		{
			name:       "returns false when member is absent from members and members_removed is empty",
			memberName: "etcd-source-0",
			createDB:   true,
			setupDB: func(t *testing.T, dbPath string) {
				db, err := bolt.Open(dbPath, 0600, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				err = db.Update(func(tx *bolt.Tx) error {
					mb, err := tx.CreateBucket([]byte("members"))
					if err != nil {
						return err
					}
					data, _ := json.Marshal(map[string]any{
						"id":       5678,
						"name":     "etcd-source-1",
						"peerURLs": []string{"http://etcd-source-1:2380"},
					})
					if err := mb.Put([]byte("0000000000005678"), data); err != nil {
						return err
					}

					_, err = tx.CreateBucket([]byte("members_removed"))
					return err
				})
				if err != nil {
					t.Fatal(err)
				}
			},
			expectedResult: false,
		},
		{
			name:       "returns true with multiple other members in bucket but current member absent",
			memberName: "etcd-source-0",
			createDB:   true,
			setupDB: func(t *testing.T, dbPath string) {
				db, err := bolt.Open(dbPath, 0600, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				err = db.Update(func(tx *bolt.Tx) error {
					mb, err := tx.CreateBucket([]byte("members"))
					if err != nil {
						return err
					}
					// Target members and remaining source members — current member absent
					for i, name := range []string{"etcd-target-0", "etcd-target-1", "etcd-target-2", "etcd-source-1", "etcd-source-2"} {
						data, _ := json.Marshal(map[string]any{
							"id":       i + 100,
							"name":     name,
							"peerURLs": []string{fmt.Sprintf("http://%s:2380", name)},
						})
						if err := mb.Put([]byte(fmt.Sprintf("%016x", i+100)), data); err != nil {
							return err
						}
					}

					rb, err := tx.CreateBucket([]byte("members_removed"))
					if err != nil {
						return err
					}
					if err := rb.Put([]byte("d4a616c047c9c84d"), []byte("removed")); err != nil {
						return err
					}
					if err := rb.Put([]byte("748c877d39cde8f9"), []byte("removed")); err != nil {
						return err
					}
					return rb.Put([]byte("db196eead3906c0c"), []byte("removed"))
				})
				if err != nil {
					t.Fatal(err)
				}
			},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			dataDir := filepath.Join(tmpDir, "data")

			if tt.createDB {
				dbDir := filepath.Join(dataDir, "member", "snap")
				if err := os.MkdirAll(dbDir, 0755); err != nil {
					t.Fatal(err)
				}
				dbPath := filepath.Join(dbDir, "db")
				if tt.setupDB != nil {
					tt.setupDB(t, dbPath)
				}
			}

			logger := logrus.New()
			m := &memberControl{
				memberName: tt.memberName,
				logger:     *logger.WithField("test", tt.name),
			}

			result, err := m.WasMemberPreviouslyRemoved(context.Background(), dataDir)

			if tt.expectedErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.expectedErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expectedResult {
				t.Errorf("expected result %v but got %v", tt.expectedResult, result)
			}
		})
	}
}
