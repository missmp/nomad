package raftutil

import (
	"fmt"
	"io"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"

	"github.com/hashicorp/nomad/helper/snapshot"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/state"
)

func RestoreFromArchive(archive io.Reader, filter *nomad.FSMFilter) (*state.StateStore, *raft.SnapshotMeta, error) {
	logger := hclog.L()

	fsm, err := dummyFSM(logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create FSM: %w", err)
	}

	r, w := io.Pipe()
	defer w.Close() // r is closed by fsm.Restore()

	errCh := make(chan error)
	metaCh := make(chan *raft.SnapshotMeta)

	go func() {
		meta, err := snapshot.CopySnapshot(archive, w)
		if err != nil {
			errCh <- fmt.Errorf("failed to read snapshot: %w", err)
		}
		if meta != nil {
			metaCh <- meta
		}
	}()

	err = fsm.RestoreFiltered(r, filter)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to restore from snapshot: %w", err)
	}

	select {
	case err := <-errCh:
		return nil, nil, err
	case meta := <-metaCh:
		return fsm.State(), meta, nil
	}
}
