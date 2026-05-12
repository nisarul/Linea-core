// SPDX-License-Identifier: AGPL-3.0-or-later

package badger_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nisarul/Linea-core/model"
	"github.com/nisarul/Linea-core/store"
	"github.com/nisarul/Linea-core/store/badger"
	"github.com/nisarul/Linea-core/store/storetest"
)

// inMemoryFactory builds an in-memory Badger Store. Disk-backed
// runs would be slow to set up per test; the on-disk path is
// covered by the round-trip test below.
func inMemoryFactory(t *testing.T) (store.Store, func()) {
	s, err := badger.Open("")
	require.NoError(t, err)
	return s, func() { _ = s.Close() }
}

func TestStore_Conformance(t *testing.T) {
	storetest.Run(t, inMemoryFactory)
}

func TestStore_OnDisk_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := badger.Open(dir)
	require.NoError(t, err)

	ctx := context.Background()
	n, err := model.NewName("Persisted", "en", "", model.NameTypeFull, true)
	require.NoError(t, err)
	p, err := model.NewPerson(model.NewID(), model.PersonOptions{Names: []model.Name{n}})
	require.NoError(t, err)

	v, err := s.Update(ctx, func(tx store.WriteTx) error { return tx.PutPerson(p) })
	require.NoError(t, err)
	require.Equal(t, store.Version(1), v)
	require.NoError(t, s.Close())

	// Reopen — version and data must survive.
	s2, err := badger.Open(dir)
	require.NoError(t, err)
	defer s2.Close()

	cur, err := s2.CurrentVersion(ctx)
	require.NoError(t, err)
	require.Equal(t, store.Version(1), cur)

	rtx, err := s2.View(ctx)
	require.NoError(t, err)
	defer rtx.Close()
	got, err := rtx.GetPerson(p.ID())
	require.NoError(t, err)
	require.Equal(t, "Persisted", got.PreferredName().Text)
}
