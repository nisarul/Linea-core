// SPDX-License-Identifier: AGPL-3.0-or-later

package badger

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"

	badgerdb "github.com/dgraph-io/badger/v4"

	lerrors "github.com/nisarul/Linea-core/errors"
	"github.com/nisarul/Linea-core/store"
)

// Store is the Badger-backed implementation of store.Store.
type Store struct {
	db *badgerdb.DB
}

// Option configures a Store at construction time.
type Option func(*badgerdb.Options)

// WithLogger replaces the default logger Badger uses internally.
func WithLogger(l badgerdb.Logger) Option {
	return func(o *badgerdb.Options) { o.Logger = l }
}

// silentLogger discards all Badger log output. Useful for tests.
type silentLogger struct{}

func (silentLogger) Errorf(string, ...any)   {}
func (silentLogger) Warningf(string, ...any) {}
func (silentLogger) Infof(string, ...any)    {}
func (silentLogger) Debugf(string, ...any)   {}

// Silent returns an Option that suppresses all Badger logging.
func Silent() Option {
	return WithLogger(silentLogger{})
}

// Open opens (or creates) a Badger-backed Store at the given
// directory path. Pass an empty path to use an in-memory Badger
// (useful for tests).
func Open(path string, opts ...Option) (*Store, error) {
	var bo badgerdb.Options
	if path == "" {
		bo = badgerdb.DefaultOptions("").WithInMemory(true)
	} else {
		bo = badgerdb.DefaultOptions(path)
	}
	bo.Logger = silentLogger{}
	for _, o := range opts {
		o(&bo)
	}
	db, err := badgerdb.Open(bo)
	if err != nil {
		return nil, fmt.Errorf("badger: open: %w", err)
	}
	s := &Store{db: db}
	if err := s.initIfEmpty(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// initIfEmpty seeds the version counter to 0 on a brand-new store.
func (s *Store) initIfEmpty() error {
	return s.db.Update(func(tx *badgerdb.Txn) error {
		_, err := tx.Get(metaVersionKey)
		if err == nil {
			return nil // already initialised
		}
		if err != badgerdb.ErrKeyNotFound {
			return err
		}
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, 0)
		return tx.Set(metaVersionKey, buf)
	})
}

// Close releases all resources held by the underlying database.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

// CurrentVersion returns the latest committed graph version.
func (s *Store) CurrentVersion(_ context.Context) (store.Version, error) {
	var v store.Version
	err := s.db.View(func(tx *badgerdb.Txn) error {
		item, err := tx.Get(metaVersionKey)
		if err != nil {
			return err
		}
		return item.Value(func(buf []byte) error {
			if len(buf) != 8 {
				return fmt.Errorf("badger: corrupt version key (len=%d)", len(buf))
			}
			v = store.Version(binary.BigEndian.Uint64(buf))
			return nil
		})
	})
	if err != nil {
		return 0, err
	}
	return v, nil
}

// View opens a read-only snapshot at the latest committed version.
func (s *Store) View(ctx context.Context) (store.ReadTx, error) {
	tx := s.db.NewTransaction(false)
	v, err := readVersion(tx)
	if err != nil {
		tx.Discard()
		return nil, err
	}
	return &readTx{tx: tx, version: v}, nil
}

// ViewAt returns a snapshot at the requested version. The Badger
// adapter only persists the current version; older versions
// return CodeVersionNotFound until snapshot history is added.
func (s *Store) ViewAt(ctx context.Context, want store.Version) (store.ReadTx, error) {
	cur, err := s.CurrentVersion(ctx)
	if err != nil {
		return nil, err
	}
	if want != cur {
		return nil, lerrors.New(lerrors.CodeVersionNotFound,
			fmt.Sprintf("badger adapter only retains current version (have %d, want %d)", cur, want))
	}
	return s.View(ctx)
}

// Update runs f inside a read-write transaction. The version is
// bumped atomically with the user's writes when f returns nil.
func (s *Store) Update(ctx context.Context, f func(tx store.WriteTx) error) (store.Version, error) {
	var newVer store.Version
	err := s.db.Update(func(btx *badgerdb.Txn) error {
		curVer, err := readVersion(btx)
		if err != nil {
			return err
		}
		w := &writeTx{readTx: readTx{tx: btx, version: curVer}}
		if err := f(w); err != nil {
			return err
		}
		newVer = curVer + 1
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(newVer))
		return btx.Set(metaVersionKey, buf)
	})
	if err != nil {
		return 0, err
	}
	return newVer, nil
}

// readVersion reads the version counter inside an existing txn.
func readVersion(tx *badgerdb.Txn) (store.Version, error) {
	item, err := tx.Get(metaVersionKey)
	if err != nil {
		return 0, fmt.Errorf("badger: read version: %w", err)
	}
	var v store.Version
	err = item.Value(func(buf []byte) error {
		if len(buf) != 8 {
			return fmt.Errorf("badger: corrupt version key (len=%d)", len(buf))
		}
		v = store.Version(binary.BigEndian.Uint64(buf))
		return nil
	})
	return v, err
}

// Compile-time interface checks.
var (
	_ store.Store   = (*Store)(nil)
	_ store.ReadTx  = (*readTx)(nil)
	_ store.WriteTx = (*writeTx)(nil)
	_ io.Closer     = (*Store)(nil)
)
