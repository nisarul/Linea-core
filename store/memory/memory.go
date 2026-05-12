// SPDX-License-Identifier: AGPL-3.0-or-later

// Package memory provides an in-memory implementation of the
// Linea Store interface. It is the canonical adapter for unit
// tests and ephemeral use; it persists nothing across process
// restarts.
//
// The implementation prioritises simplicity and correctness over
// raw throughput: each Update takes a single global write lock
// and snapshots the prior state with copy-on-write semantics so
// that concurrent ReadTx instances never observe partial writes.
package memory

import (
	"context"
	"fmt"
	"sync"

	lerrors "github.com/nisarul/Linea-core/errors"
	"github.com/nisarul/Linea-core/model"
	"github.com/nisarul/Linea-core/store"
)

// state is an immutable snapshot of the graph at a given version.
// All maps are treated as read-only once a state is published;
// mutations build a new state via copy-on-write in newWriteTx.
type state struct {
	version       store.Version
	persons       map[model.ID]model.Person
	relationships map[model.ID]model.Relationship
	sources       map[model.ID]model.Source
	proposals     map[model.ID]model.Proposal
}

func emptyState() *state {
	return &state{
		persons:       make(map[model.ID]model.Person),
		relationships: make(map[model.ID]model.Relationship),
		sources:       make(map[model.ID]model.Source),
		proposals:     make(map[model.ID]model.Proposal),
	}
}

// Store is the in-memory Store adapter.
//
// The current "live" state pointer is swapped atomically inside
// commit; readers capture the pointer they want at View time and
// hold it for the life of the ReadTx. This gives readers a
// stable snapshot without copying any data.
type Store struct {
	mu       sync.Mutex // serialises writers; readers do not contend
	current  *state
	history  map[store.Version]*state
	maxKeep  int // maximum versions kept in history (0 = unlimited)
	closed   bool
}

// Option configures a memory Store at construction time.
type Option func(*Store)

// WithHistoryLimit caps the number of historical versions kept
// available for ViewAt. Zero (the default) keeps all versions.
//
// The current version is always retained; the limit applies to
// older snapshots only.
func WithHistoryLimit(n int) Option {
	return func(s *Store) {
		if n < 0 {
			n = 0
		}
		s.maxKeep = n
	}
}

// New constructs an empty in-memory Store at version 0.
func New(opts ...Option) *Store {
	s := &Store{
		current: emptyState(),
		history: make(map[store.Version]*state),
	}
	for _, o := range opts {
		o(s)
	}
	s.history[0] = s.current
	return s
}

// CurrentVersion returns the latest committed graph version.
func (s *Store) CurrentVersion(_ context.Context) (store.Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, lerrors.New(lerrors.CodeInvalidArgument, "store is closed")
	}
	return s.current.version, nil
}

// View returns a read-only snapshot at the current version.
func (s *Store) View(_ context.Context) (store.ReadTx, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, lerrors.New(lerrors.CodeInvalidArgument, "store is closed")
	}
	return &readTx{state: s.current}, nil
}

// ViewAt returns a read-only snapshot at the requested version.
func (s *Store) ViewAt(_ context.Context, v store.Version) (store.ReadTx, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, lerrors.New(lerrors.CodeInvalidArgument, "store is closed")
	}
	st, ok := s.history[v]
	if !ok {
		return nil, lerrors.New(lerrors.CodeVersionNotFound,
			fmt.Sprintf("version %d not available", v))
	}
	return &readTx{state: st}, nil
}

// Update runs f inside a serialised read-write transaction.
// On success the transaction is committed as a new version.
func (s *Store) Update(_ context.Context, f func(tx store.WriteTx) error) (store.Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, lerrors.New(lerrors.CodeInvalidArgument, "store is closed")
	}

	tx := newWriteTx(s.current)
	if err := f(tx); err != nil {
		return 0, err
	}

	next := tx.commitState(s.current.version + 1)
	s.history[next.version] = next
	s.current = next
	s.pruneHistoryLocked()
	return next.version, nil
}

// pruneHistoryLocked enforces maxKeep. Caller must hold s.mu.
func (s *Store) pruneHistoryLocked() {
	if s.maxKeep <= 0 {
		return
	}
	cur := s.current.version
	// We always retain version 0 and the current version. All
	// intermediate versions are eligible for pruning when their
	// count exceeds maxKeep.
	keepFrom := store.Version(0)
	if cur > store.Version(s.maxKeep) {
		keepFrom = cur - store.Version(s.maxKeep)
	}
	for v := range s.history {
		if v == 0 || v == cur {
			continue
		}
		if v < keepFrom {
			delete(s.history, v)
		}
	}
}

// Close releases the Store. Subsequent calls are no-ops.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.history = nil
	return nil
}
