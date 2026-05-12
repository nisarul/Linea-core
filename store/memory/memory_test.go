// SPDX-License-Identifier: AGPL-3.0-or-later

package memory_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nisarul/Linea-core/model"
	"github.com/nisarul/Linea-core/store"
	"github.com/nisarul/Linea-core/store/memory"
	"github.com/nisarul/Linea-core/store/storetest"
)

func TestStore_Conformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) (store.Store, func()) {
		s := memory.New()
		return s, func() { _ = s.Close() }
	})
}

func TestStore_HistoryLimit(t *testing.T) {
	s := memory.New(memory.WithHistoryLimit(2))
	defer s.Close()
	ctx := context.Background()

	// Make 5 versions
	for i := 0; i < 5; i++ {
		n, err := model.NewName("p", "en", "", model.NameTypeFull, true)
		require.NoError(t, err)
		p, err := model.NewPerson(model.NewID(), model.PersonOptions{Names: []model.Name{n}})
		require.NoError(t, err)
		_, err = s.Update(ctx, func(tx store.WriteTx) error { return tx.PutPerson(p) })
		require.NoError(t, err)
	}

	// v0 (always retained) + v5 (current) MUST be readable
	rtx, err := s.ViewAt(ctx, 0)
	require.NoError(t, err)
	rtx.Close()
	rtx, err = s.ViewAt(ctx, 5)
	require.NoError(t, err)
	rtx.Close()

	// v1, v2 must have been pruned (limit 2 keeps versions 4,5 plus v0)
	_, err = s.ViewAt(ctx, 1)
	require.Error(t, err)
	_, err = s.ViewAt(ctx, 2)
	require.Error(t, err)
}

// Smoke test that concurrent writers serialise without error.
func TestStore_ConcurrentWriters(t *testing.T) {
	s := memory.New()
	defer s.Close()
	ctx := context.Background()

	const writers = 16
	var wg sync.WaitGroup
	wg.Add(writers)
	for i := 0; i < writers; i++ {
		go func() {
			defer wg.Done()
			n, _ := model.NewName("x", "en", "", model.NameTypeFull, true)
			p, _ := model.NewPerson(model.NewID(), model.PersonOptions{Names: []model.Name{n}})
			_, _ = s.Update(ctx, func(tx store.WriteTx) error { return tx.PutPerson(p) })
		}()
	}
	wg.Wait()

	v, err := s.CurrentVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, store.Version(writers), v)
}
