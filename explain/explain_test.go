// SPDX-License-Identifier: AGPL-3.0-or-later

package explain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nisarul/Linea-core/explain"
	"github.com/nisarul/Linea-core/model"
	"github.com/nisarul/Linea-core/query"
	"github.com/nisarul/Linea-core/store"
	"github.com/nisarul/Linea-core/store/memory"
)

// helpers
func mkPerson(t *testing.T, s store.Store, label string) model.ID {
	t.Helper()
	id := model.NewID()
	n, err := model.NewName(label, "en", "Latn", model.NameTypeFull, true)
	require.NoError(t, err)
	p, err := model.NewPerson(id, model.PersonOptions{Names: []model.Name{n}})
	require.NoError(t, err)
	_, err = s.Update(context.Background(), func(tx store.WriteTx) error { return tx.PutPerson(p) })
	require.NoError(t, err)
	return id
}

func mkUnknown(t *testing.T, s store.Store) model.ID {
	t.Helper()
	id := model.NewID()
	p, err := model.NewUnknownAncestor(id)
	require.NoError(t, err)
	_, err = s.Update(context.Background(), func(tx store.WriteTx) error { return tx.PutPerson(p) })
	require.NoError(t, err)
	return id
}

func mkPC(t *testing.T, s store.Store, parent, child model.ID, c model.Certainty, gapped bool) {
	t.Helper()
	cont := model.NewContinuous()
	if gapped {
		gg, err := model.KnownGap(2)
		require.NoError(t, err)
		cont = model.NewGapped(gg)
	}
	r, err := model.NewRelationship(model.NewID(), parent, child,
		model.RelTypeParentChild, c, cont, model.RelationshipOptions{})
	require.NoError(t, err)
	_, err = s.Update(context.Background(), func(tx store.WriteTx) error { return tx.PutRelationship(r) })
	require.NoError(t, err)
}

func TestExplain_PathStructure(t *testing.T) {
	s := memory.New()
	defer s.Close()
	a := mkPerson(t, s, "A")
	b := mkPerson(t, s, "B")
	c := mkPerson(t, s, "C")
	mkPC(t, s, a, b, model.CertaintyCertain, false)
	mkPC(t, s, b, c, model.CertaintyProbable, true)

	rtx, _ := s.View(context.Background())
	defer rtx.Close()

	paths, err := query.FindPaths(context.Background(), rtx, a, c, query.Options{})
	require.NoError(t, err)
	require.Len(t, paths, 1)

	exp, err := explain.Path(rtx, paths[0])
	require.NoError(t, err)
	require.Equal(t, 2, exp.Length)
	require.Equal(t, model.CertaintyProbable, exp.OverallCertainty)
	require.Equal(t, 2, exp.TotalGapGenerations)
	require.Equal(t, 1, exp.GapEdgeCount)
	require.Equal(t, query.PathLineage, exp.Classification)
	require.Equal(t, rtx.Version(), exp.GraphVersion)

	require.Len(t, exp.Edges, 2)
	require.False(t, exp.Edges[0].IsWeakestLink)
	require.True(t, exp.Edges[1].IsWeakestLink, "first edge with min cert is weakest link")
	require.Equal(t, model.CertaintyCertain, exp.Edges[0].Certainty)
	require.Equal(t, model.CertaintyProbable, exp.Edges[1].Certainty)
	require.False(t, exp.Edges[0].FromIsUnknownAncestor)
	require.False(t, exp.Edges[1].ToIsUnknownAncestor)
}

func TestExplain_NKCAFlagsUnknownPlaceholder(t *testing.T) {
	s := memory.New()
	defer s.Close()
	u := mkUnknown(t, s)
	a := mkPerson(t, s, "A")
	b := mkPerson(t, s, "B")
	mkPC(t, s, u, a, model.CertaintyProbable, false)
	mkPC(t, s, u, b, model.CertaintyProbable, false)

	rtx, _ := s.View(context.Background())
	defer rtx.Close()

	res, err := query.NearestKnownCommonAncestor(context.Background(), rtx, a, b, query.Options{})
	require.NoError(t, err)

	exp, err := explain.CommonAncestor(rtx, res)
	require.NoError(t, err)
	require.True(t, exp.AncestorIsUnknown)
	require.Equal(t, u, exp.AncestorID)
	require.Equal(t, model.CertaintyProbable, exp.CombinedCertainty)

	require.NotNil(t, exp.PathFromA)
	require.NotNil(t, exp.PathFromB)
	// On both upward paths, the ancestor end (ToPerson) is the unknown placeholder.
	last := exp.PathFromA.Edges[len(exp.PathFromA.Edges)-1]
	require.True(t, last.ToIsUnknownAncestor)
}
