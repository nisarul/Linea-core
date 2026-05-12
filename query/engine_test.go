// SPDX-License-Identifier: AGPL-3.0-or-later

package query_test

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	lerrors "github.com/nisarul/Linea-core/errors"
	"github.com/nisarul/Linea-core/model"
	"github.com/nisarul/Linea-core/query"
	"github.com/nisarul/Linea-core/store"
	"github.com/nisarul/Linea-core/store/memory"
)

// builder helps assemble a small graph in tests.
type graph struct {
	t       *testing.T
	store   store.Store
	persons map[string]model.ID
}

func newGraph(t *testing.T) *graph {
	t.Helper()
	return &graph{
		t:       t,
		store:   memory.New(),
		persons: map[string]model.ID{},
	}
}

func (g *graph) addPerson(label string) model.ID {
	g.t.Helper()
	id := model.NewID()
	g.persons[label] = id
	n, err := model.NewName(label, "en", "Latn", model.NameTypeFull, true)
	require.NoError(g.t, err)
	p, err := model.NewPerson(id, model.PersonOptions{Names: []model.Name{n}})
	require.NoError(g.t, err)
	_, err = g.store.Update(context.Background(), func(tx store.WriteTx) error {
		return tx.PutPerson(p)
	})
	require.NoError(g.t, err)
	return id
}

func (g *graph) addUnknown(label string) model.ID {
	g.t.Helper()
	id := model.NewID()
	g.persons[label] = id
	p, err := model.NewUnknownAncestor(id)
	require.NoError(g.t, err)
	_, err = g.store.Update(context.Background(), func(tx store.WriteTx) error {
		return tx.PutPerson(p)
	})
	require.NoError(g.t, err)
	return id
}

func (g *graph) addPC(parent, child string, c model.Certainty, gapped bool, gap int) {
	g.t.Helper()
	cont := model.NewContinuous()
	if gapped {
		if gap > 0 {
			gg, err := model.KnownGap(gap)
			require.NoError(g.t, err)
			cont = model.NewGapped(gg)
		} else {
			cont = model.NewGapped(model.UnknownGap())
		}
	}
	r, err := model.NewRelationship(model.NewID(), g.persons[parent], g.persons[child],
		model.RelTypeParentChild, c, cont, model.RelationshipOptions{})
	require.NoError(g.t, err)
	_, err = g.store.Update(context.Background(), func(tx store.WriteTx) error {
		return tx.PutRelationship(r)
	})
	require.NoError(g.t, err)
}

func (g *graph) addMarriage(a, b string) {
	g.t.Helper()
	r, err := model.NewRelationship(model.NewID(), g.persons[a], g.persons[b],
		model.RelTypeMarriage, model.CertaintyCertain, model.NewContinuous(), model.RelationshipOptions{})
	require.NoError(g.t, err)
	_, err = g.store.Update(context.Background(), func(tx store.WriteTx) error {
		return tx.PutRelationship(r)
	})
	require.NoError(g.t, err)
}

func (g *graph) view() store.ReadTx {
	g.t.Helper()
	rtx, err := g.store.View(context.Background())
	require.NoError(g.t, err)
	return rtx
}

func (g *graph) close() { _ = g.store.Close() }

// ---- Tests ----

func TestFindPaths_NoConnection(t *testing.T) {
	g := newGraph(t)
	defer g.close()
	a := g.addPerson("A")
	b := g.addPerson("B")
	rtx := g.view()
	defer rtx.Close()

	_, err := query.FindPaths(context.Background(), rtx, a, b, query.Options{})
	require.Error(t, err)
	require.True(t, lerrors.IsNoKnownConnection(err), "expected NO_KNOWN_CONNECTION, got %v", err)
}

func TestFindPaths_DirectParentChild(t *testing.T) {
	g := newGraph(t)
	defer g.close()
	parent := g.addPerson("Parent")
	child := g.addPerson("Child")
	g.addPC("Parent", "Child", model.CertaintyCertain, false, 0)
	rtx := g.view()
	defer rtx.Close()

	paths, err := query.FindPaths(context.Background(), rtx, parent, child, query.Options{})
	require.NoError(t, err)
	require.Len(t, paths, 1)
	require.Equal(t, 1, paths[0].Length)
	require.Equal(t, query.PathLineage, paths[0].Classification)
	require.Equal(t, model.CertaintyCertain, paths[0].Certainty)
	require.Equal(t, rtx.Version(), paths[0].GraphVersion)
}

func TestFindPaths_RankingPrefersHigherCertainty(t *testing.T) {
	g := newGraph(t)
	defer g.close()
	// A -> X -> C  (Probable, Probable)        weakest=Probable
	// A -> Y -> C  (Certain, Certain)          weakest=Certain
	g.addPerson("A")
	g.addPerson("X")
	g.addPerson("Y")
	g.addPerson("C")
	g.addPC("A", "X", model.CertaintyProbable, false, 0)
	g.addPC("X", "C", model.CertaintyProbable, false, 0)
	g.addPC("A", "Y", model.CertaintyCertain, false, 0)
	g.addPC("Y", "C", model.CertaintyCertain, false, 0)

	rtx := g.view()
	defer rtx.Close()

	paths, err := query.FindPaths(context.Background(), rtx, g.persons["A"], g.persons["C"], query.Options{})
	require.NoError(t, err)
	require.Len(t, paths, 2)
	require.Equal(t, model.CertaintyCertain, paths[0].Certainty,
		"highest certainty path must come first")
}

func TestFindPaths_RankingTotalGapBeforeLength(t *testing.T) {
	g := newGraph(t)
	defer g.close()
	// Two equally-certain routes from A to C:
	//   A -gap5-> C       (length 1, total gap 5)
	//   A -> X -> C       (length 2, total gap 0)
	// per CCGGS §9.3, smaller total gap wins over shorter length
	g.addPerson("A")
	g.addPerson("X")
	g.addPerson("C")
	g.addPC("A", "C", model.CertaintyCertain, true, 5)
	g.addPC("A", "X", model.CertaintyCertain, false, 0)
	g.addPC("X", "C", model.CertaintyCertain, false, 0)

	rtx := g.view()
	defer rtx.Close()

	paths, err := query.FindPaths(context.Background(), rtx, g.persons["A"], g.persons["C"], query.Options{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(paths), 2)
	require.Equal(t, 0, paths[0].TotalGap, "smaller total gap must come first")
	require.Equal(t, 2, paths[0].Length)
}

func TestFindPaths_LineagePreferredOverAffinal(t *testing.T) {
	g := newGraph(t)
	defer g.close()
	// Affinal route: A -[m]- B  (when IncludeAffinal=true)
	// Lineage route: A -> B (parent->child)
	g.addPerson("A")
	g.addPerson("B")
	g.addPC("A", "B", model.CertaintyCertain, false, 0)
	g.addMarriage("A", "B") // unusual but tests classification

	rtx := g.view()
	defer rtx.Close()

	paths, err := query.FindPaths(context.Background(), rtx, g.persons["A"], g.persons["B"],
		query.Options{IncludeAffinal: true})
	require.NoError(t, err)
	require.Equal(t, query.PathLineage, paths[0].Classification,
		"lineage paths must come before affinal paths")
}

func TestFindPaths_AffinalDisabledByDefault(t *testing.T) {
	g := newGraph(t)
	defer g.close()
	g.addPerson("A")
	g.addPerson("B")
	g.addMarriage("A", "B")
	rtx := g.view()
	defer rtx.Close()

	_, err := query.FindPaths(context.Background(), rtx, g.persons["A"], g.persons["B"], query.Options{})
	require.True(t, lerrors.IsNoKnownConnection(err))

	paths, err := query.FindPaths(context.Background(), rtx, g.persons["A"], g.persons["B"],
		query.Options{IncludeAffinal: true})
	require.NoError(t, err)
	require.Equal(t, query.PathAffinal, paths[0].Classification)
}

func TestFindPaths_WeakestLinkPropagation(t *testing.T) {
	g := newGraph(t)
	defer g.close()
	// A -[Certain]-> B -[Uncertain]-> C
	g.addPerson("A")
	g.addPerson("B")
	g.addPerson("C")
	g.addPC("A", "B", model.CertaintyCertain, false, 0)
	g.addPC("B", "C", model.CertaintyUncertain, false, 0)

	rtx := g.view()
	defer rtx.Close()
	paths, err := query.FindPaths(context.Background(), rtx, g.persons["A"], g.persons["C"], query.Options{})
	require.NoError(t, err)
	require.Equal(t, model.CertaintyUncertain, paths[0].Certainty)
}

func TestFindPaths_NotFoundForUnknownPerson(t *testing.T) {
	g := newGraph(t)
	defer g.close()
	a := g.addPerson("A")
	rtx := g.view()
	defer rtx.Close()
	_, err := query.FindPaths(context.Background(), rtx, a, model.NewID(), query.Options{})
	require.True(t, lerrors.HasCode(err, lerrors.CodePersonNotFound))
}

func TestNKCA_BasicCousins(t *testing.T) {
	g := newGraph(t)
	defer g.close()
	// Tree:
	//   GP -> P1 -> A
	//   GP -> P2 -> B
	g.addPerson("GP")
	g.addPerson("P1")
	g.addPerson("P2")
	g.addPerson("A")
	g.addPerson("B")
	g.addPC("GP", "P1", model.CertaintyCertain, false, 0)
	g.addPC("GP", "P2", model.CertaintyCertain, false, 0)
	g.addPC("P1", "A", model.CertaintyCertain, false, 0)
	g.addPC("P2", "B", model.CertaintyCertain, false, 0)

	rtx := g.view()
	defer rtx.Close()
	res, err := query.NearestKnownCommonAncestor(context.Background(), rtx,
		g.persons["A"], g.persons["B"], query.Options{})
	require.NoError(t, err)
	require.Equal(t, g.persons["GP"], res.AncestorID)
	require.Equal(t, 4, res.TotalGenerations) // 2 + 2
	require.False(t, res.Unknown)
}

func TestNKCA_PrefersFewerGenerationsThenStableID(t *testing.T) {
	g := newGraph(t)
	defer g.close()
	// Two common ancestors at the same depth: NKCA tiebreak by id
	//   X -> A
	//   X -> B
	//   Y -> A
	//   Y -> B
	g.addPerson("X")
	g.addPerson("Y")
	g.addPerson("A")
	g.addPerson("B")
	g.addPC("X", "A", model.CertaintyCertain, false, 0)
	g.addPC("X", "B", model.CertaintyCertain, false, 0)
	g.addPC("Y", "A", model.CertaintyCertain, false, 0)
	g.addPC("Y", "B", model.CertaintyCertain, false, 0)

	rtx := g.view()
	defer rtx.Close()
	res, err := query.NearestKnownCommonAncestor(context.Background(), rtx,
		g.persons["A"], g.persons["B"], query.Options{})
	require.NoError(t, err)
	candidates := []model.ID{g.persons["X"], g.persons["Y"]}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i] < candidates[j] })
	require.Equal(t, candidates[0], res.AncestorID, "smallest-id tie wins")
}

func TestNKCA_UnknownAncestorPlaceholder(t *testing.T) {
	g := newGraph(t)
	defer g.close()
	// Known siblings A and B with an undocumented shared parent (unknown placeholder)
	g.addUnknown("U")
	g.addPerson("A")
	g.addPerson("B")
	g.addPC("U", "A", model.CertaintyProbable, false, 0)
	g.addPC("U", "B", model.CertaintyProbable, false, 0)

	rtx := g.view()
	defer rtx.Close()
	res, err := query.NearestKnownCommonAncestor(context.Background(), rtx,
		g.persons["A"], g.persons["B"], query.Options{})
	require.NoError(t, err)
	require.True(t, res.Unknown, "NKCA must be flagged as unknown placeholder")
	require.Equal(t, g.persons["U"], res.AncestorID)
}

func TestNKCA_NoConnection(t *testing.T) {
	g := newGraph(t)
	defer g.close()
	g.addPerson("A")
	g.addPerson("B")
	rtx := g.view()
	defer rtx.Close()
	_, err := query.NearestKnownCommonAncestor(context.Background(), rtx,
		g.persons["A"], g.persons["B"], query.Options{})
	require.True(t, lerrors.IsNoKnownConnection(err))
}
