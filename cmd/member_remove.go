// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gardener/etcd-backup-restore/pkg/etcdutil"
	brtypes "github.com/gardener/etcd-backup-restore/pkg/types"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NewMemberRemoveCommand creates a cobra command for removing members from an etcd cluster.
func NewMemberRemoveCommand(_ context.Context) *cobra.Command {
	opts := &memberRemoveOptions{
		etcdConnConfig: brtypes.NewEtcdConnectionConfig(),
	}

	cmd := &cobra.Command{
		Use:   "member-remove",
		Short: "removes one or more members from an etcd cluster",
		Long: `Removes the specified members from an etcd cluster by their name and peer URL.
Each member is removed sequentially with a health check between removals to ensure
cluster stability. Already-absent members are treated as no-ops.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			logger := logrus.New()
			if err := opts.validate(); err != nil {
				return fmt.Errorf("invalid options: %w", err)
			}
			return runMemberRemove(logger, opts)
		},
	}

	cmd.Flags().StringVar(&opts.members, "members", "", "comma-separated list of members to remove in format name=peerURL (e.g. etcd-0=http://etcd-0.peer:2380,etcd-1=http://etcd-1.peer:2380)")
	opts.etcdConnConfig.AddFlags(cmd.Flags())

	return cmd
}

type memberRemoveOptions struct {
	members        string
	etcdConnConfig *brtypes.EtcdConnectionConfig
}

type memberToRemove struct {
	name    string
	peerURL string
}

func (o *memberRemoveOptions) validate() error {
	if o.members == "" {
		return fmt.Errorf("--members is required")
	}
	if _, err := parseMembersFlag(o.members); err != nil {
		return err
	}
	return nil
}

func parseMembersFlag(members string) ([]memberToRemove, error) {
	parts := strings.Split(members, ",")
	result := make([]memberToRemove, 0, len(parts))
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return nil, fmt.Errorf("invalid member format %q, expected name=peerURL", part)
		}
		result = append(result, memberToRemove{name: kv[0], peerURL: kv[1]})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("at least one member must be specified")
	}
	return result, nil
}

func runMemberRemove(logger *logrus.Logger, opts *memberRemoveOptions) error {
	members, _ := parseMembersFlag(opts.members)
	entry := logrus.NewEntry(logger)

	factory := etcdutil.NewFactory(*opts.etcdConnConfig)

	for i, m := range members {
		entry.Infof("Removing member %d/%d: %s (peerURL: %s)", i+1, len(members), m.name, m.peerURL)

		cli, err := factory.NewCluster()
		if err != nil {
			return fmt.Errorf("failed to create etcd cluster client: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		memberList, err := cli.MemberList(ctx)
		cancel()
		if err != nil {
			cli.Close()
			return fmt.Errorf("failed to list members: %w", err)
		}

		var memberID uint64
		var found bool
		for _, existing := range memberList.Members {
			for _, peerURL := range existing.PeerURLs {
				if peerURL == m.peerURL {
					memberID = existing.ID
					found = true
					break
				}
			}
			if found {
				break
			}
			if existing.Name == m.name {
				memberID = existing.ID
				found = true
				break
			}
		}

		if !found {
			entry.Infof("Member %s not found in cluster (already removed), skipping", m.name)
			cli.Close()
			continue
		}

		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		_, err = cli.MemberRemove(ctx, memberID)
		cancel()
		cli.Close()

		if err != nil {
			if strings.Contains(err.Error(), "member not found") {
				entry.Infof("Member %s (ID: %x) already removed, skipping", m.name, memberID)
				continue
			}
			return fmt.Errorf("failed to remove member %s (ID: %x): %w", m.name, memberID, err)
		}

		entry.Infof("Successfully removed member %s (ID: %x)", m.name, memberID)

		if i < len(members)-1 {
			entry.Info("Waiting 2s for cluster to stabilize before next removal...")
			time.Sleep(2 * time.Second)
		}
	}

	entry.Info("All member removals completed successfully")
	return nil
}
