// SPDX-License-Identifier: AGPL-3.0-or-later

// Package storetest provides a portable conformance test suite
// that any Store adapter MUST pass.
//
// Adapter packages plug into it like:
//
//	func TestStore_Conformance(t *testing.T) {
//	    storetest.Run(t, func(t *testing.T) (store.Store, func()) {
//	        s := myadapter.New(...)
//	        return s, func() { _ = s.Close() }
//	    })
//	}
package storetest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	lerrors "github.com/nisarul/Linea-core/errors"
	"github.com/nisarul/Linea-core/model"
	"github.com/nisarul/Linea-core/store"
)

// Factory builds a fresh Store and a cleanup function.
type Factory func(t *testing.T) (store.Store, func())

// Run executes the full conformance suite against the supplied factory.
func Run(t *testing.T, factory Factory) {
	t.Helper()
	cases := []struct {
		name string
		fn   func(*testing.T, store.Store)
	}{
		{"InitialVersionIsZero", testInitialVersion},
		{"PersonRoundTrip", testPersonRoundTrip},
		{"RelationshipRoundTrip", testRelationshipRoundTrip},
		{"SourceRoundTrip", testSourceRoundTrip},
		{"ProposalRoundTrip", testProposalRoundTrip},
		{"VersioningMonotonic", testVersioning},
		{"SnapshotIsolation", testSnapshotIsolation},
		{"RelationshipMissingEndpoint", testRelMissingEndpoint},
		{"DeleteRoundTrip", testDeletes},
		{"NotFoundCodes", testNotFoundCodes},
		{"IteratorEarlyTermination", testIteratorEarlyTerm},
		{"ParentChildIteration", testParentChildIteration},
		{"MarriageIteration", testMarriageIteration},
		{"SourceIteration", testSourceIteration},
		{"RollbackOnError", testRollback},
		{"TerminalProposalImmutable", testTerminalProposal},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			s, cleanup := factory(t)
			defer cleanup()
			c.fn(t, s)
		})
	}
}

// ---------- helpers ----------

func ctx() context.Context { return context.Background() }

func mustPerson(t *testing.T, name string) model.Person {
	t.Helper()
	n, err := model.NewName(name, "en", "Latn", model.NameTypeFull, true)
	require.NoError(t, err)
	p, err := model.NewPerson(model.NewID(), model.PersonOptions{Names: []model.Name{n}})
	require.NoError(t, err)
	return p
}

func mustParentChild(t *testing.T, parent, child model.ID, c model.Certainty) model.Relationship {
	t.Helper()
	r, err := model.NewRelationship(model.NewID(), parent, child, model.RelTypeParentChild,
		c, model.NewContinuous(), model.RelationshipOptions{})
	require.NoError(t, err)
	return r
}

func mustMarriage(t *testing.T, a, b model.ID) model.Relationship {
	t.Helper()
	r, err := model.NewRelationship(model.NewID(), a, b, model.RelTypeMarriage,
		model.CertaintyCertain, model.NewContinuous(), model.RelationshipOptions{})
	require.NoError(t, err)
	return r
}

func mustSource(t *testing.T) model.Source {
	t.Helper()
	s, err := model.NewSource(model.NewID(), model.SourceTypePrimary, "Test citation",
		model.SourceOptions{Author: "Anon"})
	require.NoError(t, err)
	return s
}

func mustProposal(t *testing.T) model.Proposal {
	t.Helper()
	p, err := model.NewProposal(model.NewID(), model.ProposalActionCreate, model.EntityKindPerson,
		model.ProposalOptions{Reason: "init", Author: "tester"})
	require.NoError(t, err)
	return p
}

// ---------- tests ----------

func testInitialVersion(t *testing.T, s store.Store) {
	v, err := s.CurrentVersion(ctx())
	require.NoError(t, err)
	require.Equal(t, store.Version(0), v)
}

func testPersonRoundTrip(t *testing.T, s store.Store) {
	alice := mustPerson(t, "Alice")
	v, err := s.Update(ctx(), func(tx store.WriteTx) error {
		return tx.PutPerson(alice)
	})
	require.NoError(t, err)
	require.Equal(t, store.Version(1), v)

	rtx, err := s.View(ctx())
	require.NoError(t, err)
	defer rtx.Close()
	got, err := rtx.GetPerson(alice.ID())
	require.NoError(t, err)
	require.Equal(t, alice.ID(), got.ID())
	require.Equal(t, "Alice", got.PreferredName().Text)
}

func testRelationshipRoundTrip(t *testing.T, s store.Store) {
	parent := mustPerson(t, "Parent")
	child := mustPerson(t, "Child")
	r := mustParentChild(t, parent.ID(), child.ID(), model.CertaintyCertain)

	_, err := s.Update(ctx(), func(tx store.WriteTx) error {
		if err := tx.PutPerson(parent); err != nil {
			return err
		}
		if err := tx.PutPerson(child); err != nil {
			return err
		}
		return tx.PutRelationship(r)
	})
	require.NoError(t, err)

	rtx, _ := s.View(ctx())
	defer rtx.Close()
	got, err := rtx.GetRelationship(r.ID())
	require.NoError(t, err)
	require.Equal(t, r.ID(), got.ID())
	require.Equal(t, model.RelTypeParentChild, got.Type())
	require.Equal(t, parent.ID(), got.From())
	require.Equal(t, child.ID(), got.To())
}

func testSourceRoundTrip(t *testing.T, s store.Store) {
	src := mustSource(t)
	_, err := s.Update(ctx(), func(tx store.WriteTx) error { return tx.PutSource(src) })
	require.NoError(t, err)

	rtx, _ := s.View(ctx())
	defer rtx.Close()
	got, err := rtx.GetSource(src.ID())
	require.NoError(t, err)
	require.Equal(t, src.ID(), got.ID())
	require.Equal(t, model.SourceTypePrimary, got.Type())
}

func testProposalRoundTrip(t *testing.T, s store.Store) {
	p := mustProposal(t)
	_, err := s.Update(ctx(), func(tx store.WriteTx) error { return tx.PutProposal(p) })
	require.NoError(t, err)

	rtx, _ := s.View(ctx())
	defer rtx.Close()
	got, err := rtx.GetProposal(p.ID())
	require.NoError(t, err)
	require.Equal(t, model.ProposalDraft, got.State())
}

func testVersioning(t *testing.T, s store.Store) {
	for i := 1; i <= 5; i++ {
		v, err := s.Update(ctx(), func(tx store.WriteTx) error {
			return tx.PutPerson(mustPerson(t, "P"))
		})
		require.NoError(t, err)
		require.Equal(t, store.Version(i), v)
	}
	cur, err := s.CurrentVersion(ctx())
	require.NoError(t, err)
	require.Equal(t, store.Version(5), cur)
}

func testSnapshotIsolation(t *testing.T, s store.Store) {
	// View at v0 must not observe writes that came later.
	v0, err := s.View(ctx())
	require.NoError(t, err)
	defer v0.Close()
	require.Equal(t, store.Version(0), v0.Version())

	alice := mustPerson(t, "Alice")
	_, err = s.Update(ctx(), func(tx store.WriteTx) error { return tx.PutPerson(alice) })
	require.NoError(t, err)

	_, err = v0.GetPerson(alice.ID())
	require.Error(t, err)
	require.True(t, lerrors.HasCode(err, lerrors.CodePersonNotFound))

	// Newer view sees the write.
	v1, err := s.View(ctx())
	require.NoError(t, err)
	defer v1.Close()
	_, err = v1.GetPerson(alice.ID())
	require.NoError(t, err)
}

func testRelMissingEndpoint(t *testing.T, s store.Store) {
	missing := model.NewID()
	known := mustPerson(t, "Known")
	r := mustParentChild(t, missing, known.ID(), model.CertaintyCertain)

	_, err := s.Update(ctx(), func(tx store.WriteTx) error {
		if err := tx.PutPerson(known); err != nil {
			return err
		}
		return tx.PutRelationship(r)
	})
	require.Error(t, err)
	require.True(t, lerrors.HasCode(err, lerrors.CodePersonNotFound),
		"expected PersonNotFound, got %v", err)
}

func testDeletes(t *testing.T, s store.Store) {
	a := mustPerson(t, "A")
	b := mustPerson(t, "B")
	r := mustParentChild(t, a.ID(), b.ID(), model.CertaintyProbable)

	_, err := s.Update(ctx(), func(tx store.WriteTx) error {
		_ = tx.PutPerson(a)
		_ = tx.PutPerson(b)
		return tx.PutRelationship(r)
	})
	require.NoError(t, err)

	_, err = s.Update(ctx(), func(tx store.WriteTx) error {
		return tx.DeleteRelationship(r.ID())
	})
	require.NoError(t, err)

	rtx, _ := s.View(ctx())
	defer rtx.Close()
	_, err = rtx.GetRelationship(r.ID())
	require.True(t, lerrors.HasCode(err, lerrors.CodeRelationshipNotFound))

	// Deleting again returns NotFound
	_, err = s.Update(ctx(), func(tx store.WriteTx) error {
		return tx.DeleteRelationship(r.ID())
	})
	require.True(t, lerrors.HasCode(err, lerrors.CodeRelationshipNotFound))
}

func testNotFoundCodes(t *testing.T, s store.Store) {
	rtx, _ := s.View(ctx())
	defer rtx.Close()
	missing := model.NewID()

	_, err := rtx.GetPerson(missing)
	require.True(t, lerrors.HasCode(err, lerrors.CodePersonNotFound))
	_, err = rtx.GetRelationship(missing)
	require.True(t, lerrors.HasCode(err, lerrors.CodeRelationshipNotFound))
	_, err = rtx.GetSource(missing)
	require.True(t, lerrors.HasCode(err, lerrors.CodeSourceNotFound))
	_, err = rtx.GetProposal(missing)
	require.True(t, lerrors.HasCode(err, lerrors.CodeProposalNotFound))
}

func testIteratorEarlyTerm(t *testing.T, s store.Store) {
	_, err := s.Update(ctx(), func(tx store.WriteTx) error {
		for i := 0; i < 5; i++ {
			if err := tx.PutPerson(mustPerson(t, "p")); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err)

	rtx, _ := s.View(ctx())
	defer rtx.Close()

	count := 0
	err = rtx.IteratePersons(func(model.Person) bool {
		count++
		return count < 3 // stop after 3
	})
	require.NoError(t, err)
	require.Equal(t, 3, count)
}

func testParentChildIteration(t *testing.T, s store.Store) {
	parent := mustPerson(t, "Parent")
	child1 := mustPerson(t, "C1")
	child2 := mustPerson(t, "C2")
	other := mustPerson(t, "Other")

	r1 := mustParentChild(t, parent.ID(), child1.ID(), model.CertaintyCertain)
	r2 := mustParentChild(t, parent.ID(), child2.ID(), model.CertaintyCertain)
	r3 := mustParentChild(t, other.ID(), child1.ID(), model.CertaintyCertain)

	_, err := s.Update(ctx(), func(tx store.WriteTx) error {
		_ = tx.PutPerson(parent)
		_ = tx.PutPerson(child1)
		_ = tx.PutPerson(child2)
		_ = tx.PutPerson(other)
		_ = tx.PutRelationship(r1)
		_ = tx.PutRelationship(r2)
		return tx.PutRelationship(r3)
	})
	require.NoError(t, err)

	rtx, _ := s.View(ctx())
	defer rtx.Close()

	var children []model.ID
	require.NoError(t, rtx.IterateChildren(parent.ID(), func(r model.Relationship) bool {
		children = append(children, r.To())
		return true
	}))
	require.ElementsMatch(t, []model.ID{child1.ID(), child2.ID()}, children)

	var parents []model.ID
	require.NoError(t, rtx.IterateParents(child1.ID(), func(r model.Relationship) bool {
		parents = append(parents, r.From())
		return true
	}))
	require.ElementsMatch(t, []model.ID{parent.ID(), other.ID()}, parents)
}

func testMarriageIteration(t *testing.T, s store.Store) {
	a := mustPerson(t, "A")
	b := mustPerson(t, "B")
	c := mustPerson(t, "C")
	mab := mustMarriage(t, a.ID(), b.ID())
	mac := mustMarriage(t, a.ID(), c.ID())

	_, err := s.Update(ctx(), func(tx store.WriteTx) error {
		_ = tx.PutPerson(a)
		_ = tx.PutPerson(b)
		_ = tx.PutPerson(c)
		_ = tx.PutRelationship(mab)
		return tx.PutRelationship(mac)
	})
	require.NoError(t, err)

	rtx, _ := s.View(ctx())
	defer rtx.Close()

	var partners []model.ID
	require.NoError(t, rtx.IterateMarriages(a.ID(), func(r model.Relationship) bool {
		other := r.To()
		if other == a.ID() {
			other = r.From()
		}
		partners = append(partners, other)
		return true
	}))
	require.ElementsMatch(t, []model.ID{b.ID(), c.ID()}, partners)
}

func testSourceIteration(t *testing.T, s store.Store) {
	a := mustSource(t)
	b := mustSource(t)
	c := mustSource(t)
	_, err := s.Update(ctx(), func(tx store.WriteTx) error {
		for _, src := range []model.Source{a, b, c} {
			if err := tx.PutSource(src); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err)

	rtx, _ := s.View(ctx())
	defer rtx.Close()
	var ids []model.ID
	require.NoError(t, rtx.IterateSources(func(src model.Source) bool {
		ids = append(ids, src.ID())
		return true
	}))
	require.ElementsMatch(t, []model.ID{a.ID(), b.ID(), c.ID()}, ids)
}

func testRollback(t *testing.T, s store.Store) {
	beforeV, err := s.CurrentVersion(ctx())
	require.NoError(t, err)

	sentinel := errors.New("rollback me")
	_, err = s.Update(ctx(), func(tx store.WriteTx) error {
		_ = tx.PutPerson(mustPerson(t, "Ghost"))
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)

	afterV, err := s.CurrentVersion(ctx())
	require.NoError(t, err)
	require.Equal(t, beforeV, afterV, "failed Update must not advance version")
}

func testTerminalProposal(t *testing.T, s store.Store) {
	p := mustProposal(t)
	_, err := s.Update(ctx(), func(tx store.WriteTx) error { return tx.PutProposal(p) })
	require.NoError(t, err)

	// Move to Accepted (terminal)
	accepted := p.WithStateUnchecked(model.ProposalAccepted, model.ProposalTransition{
		From: model.ProposalDraft, To: model.ProposalAccepted,
		Actor: "curator", Timestamp: 1,
	})
	_, err = s.Update(ctx(), func(tx store.WriteTx) error { return tx.PutProposal(accepted) })
	require.NoError(t, err)

	// Attempt to mutate to a different state → must be rejected
	rejectedAttempt := accepted.WithStateUnchecked(model.ProposalRejected, model.ProposalTransition{
		From: model.ProposalAccepted, To: model.ProposalRejected,
		Actor: "curator", Timestamp: 2, Reason: "oops",
	})
	_, err = s.Update(ctx(), func(tx store.WriteTx) error { return tx.PutProposal(rejectedAttempt) })
	require.Error(t, err)
	require.True(t, lerrors.HasCode(err, lerrors.CodeImmutableTerminalProposal),
		"expected ImmutableTerminalProposal, got %v", err)
}
