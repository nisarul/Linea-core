// SPDX-License-Identifier: AGPL-3.0-or-later

package governance_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	lerrors "github.com/nisarul/Linea-core/errors"
	"github.com/nisarul/Linea-core/governance"
	"github.com/nisarul/Linea-core/model"
	"github.com/nisarul/Linea-core/store"
	"github.com/nisarul/Linea-core/store/memory"
)

func newStore(t *testing.T) (store.Store, context.Context) {
	t.Helper()
	return memory.New(), context.Background()
}

// helper: seed a Draft Proposal of the given action/kind.
func seedProposal(t *testing.T, s store.Store, ctx context.Context,
	action model.ProposalAction, kind model.EntityKind, opts model.ProposalOptions,
) model.Proposal {
	t.Helper()
	p, err := model.NewProposal(model.NewID(), action, kind, opts)
	require.NoError(t, err)
	_, err = s.Update(ctx, func(tx store.WriteTx) error { return tx.PutProposal(p) })
	require.NoError(t, err)
	return p
}

func encode(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestStateMachine_LegalTransitions(t *testing.T) {
	require.True(t, governance.CanTransition(model.ProposalDraft, model.ProposalSubmitted))
	require.True(t, governance.CanTransition(model.ProposalDraft, model.ProposalWithdrawn))
	require.True(t, governance.CanTransition(model.ProposalSubmitted, model.ProposalUnderReview))
	require.True(t, governance.CanTransition(model.ProposalSubmitted, model.ProposalWithdrawn))
	require.True(t, governance.CanTransition(model.ProposalUnderReview, model.ProposalAccepted))
	require.True(t, governance.CanTransition(model.ProposalUnderReview, model.ProposalRejected))
	require.True(t, governance.CanTransition(model.ProposalUnderReview, model.ProposalWithdrawn))

	require.False(t, governance.CanTransition(model.ProposalAccepted, model.ProposalRejected))
	require.False(t, governance.CanTransition(model.ProposalRejected, model.ProposalAccepted))
	require.False(t, governance.CanTransition(model.ProposalWithdrawn, model.ProposalSubmitted))

	// Skipping a state is not allowed
	require.False(t, governance.CanTransition(model.ProposalDraft, model.ProposalUnderReview))
	require.False(t, governance.CanTransition(model.ProposalDraft, model.ProposalAccepted))
}

func TestTransition_RejectRequiresReason(t *testing.T) {
	p, err := model.NewProposal(model.NewID(), model.ProposalActionCreate, model.EntityKindPerson,
		model.ProposalOptions{})
	require.NoError(t, err)
	p = p.WithStateUnchecked(model.ProposalUnderReview, model.ProposalTransition{
		From: model.ProposalDraft, To: model.ProposalUnderReview,
	})
	_, err = governance.Transition(p, model.ProposalRejected, "alice", 1, "")
	require.Error(t, err)
	require.True(t, lerrors.HasCode(err, lerrors.CodeInvalidArgument))

	out, err := governance.Transition(p, model.ProposalRejected, "alice", 2, "needs better source")
	require.NoError(t, err)
	require.Equal(t, model.ProposalRejected, out.State())
	require.Equal(t, 2, len(out.History()))
}

func TestEndToEnd_CreatePerson(t *testing.T) {
	s, ctx := newStore(t)
	defer s.Close()

	pl := governance.PayloadCreatePerson{
		Names: []model.Name{{Text: "Alice", Type: model.NameTypeGiven, Preferred: true}},
	}
	p := seedProposal(t, s, ctx, model.ProposalActionCreate, model.EntityKindPerson,
		model.ProposalOptions{Payload: encode(t, pl), Reason: "create alice", Author: "alice"})

	_, err := governance.Submit(ctx, s, p.ID(), "alice", 10)
	require.NoError(t, err)
	_, err = governance.Claim(ctx, s, p.ID(), "curator", 11)
	require.NoError(t, err)
	out, err := governance.Accept(ctx, s, p.ID(), "curator", 12)
	require.NoError(t, err)
	require.Equal(t, model.ProposalAccepted, out.State())
	require.Equal(t, 3, len(out.History())) // submit, claim, accept

	// Person should now exist (we don't know the ID, so iterate)
	rtx, err := s.View(ctx)
	require.NoError(t, err)
	defer rtx.Close()
	var found bool
	require.NoError(t, rtx.IteratePersons(func(person model.Person) bool {
		if person.PreferredName().Text == "Alice" {
			found = true
			return false
		}
		return true
	}))
	require.True(t, found, "Alice was not created")
}

func TestEndToEnd_RejectKeepsGraphIntact(t *testing.T) {
	s, ctx := newStore(t)
	defer s.Close()

	pl := governance.PayloadCreatePerson{
		Names: []model.Name{{Text: "Bob", Type: model.NameTypeGiven, Preferred: true}},
	}
	p := seedProposal(t, s, ctx, model.ProposalActionCreate, model.EntityKindPerson,
		model.ProposalOptions{Payload: encode(t, pl)})

	_, err := governance.Submit(ctx, s, p.ID(), "bob", 1)
	require.NoError(t, err)
	_, err = governance.Claim(ctx, s, p.ID(), "curator", 2)
	require.NoError(t, err)
	_, err = governance.Reject(ctx, s, p.ID(), "curator", 3, "duplicate")
	require.NoError(t, err)

	rtx, _ := s.View(ctx)
	defer rtx.Close()
	count := 0
	require.NoError(t, rtx.IteratePersons(func(model.Person) bool { count++; return true }))
	require.Equal(t, 0, count, "rejected proposal must not create entities")
}

func TestUnknownAncestor_RejectsFabrication(t *testing.T) {
	s, ctx := newStore(t)
	defer s.Close()

	pl := governance.PayloadCreatePerson{
		UnknownAncestor: true,
		Names: []model.Name{{Text: "Made Up", Type: model.NameTypeGiven, Preferred: true}},
	}
	p := seedProposal(t, s, ctx, model.ProposalActionCreate, model.EntityKindPerson,
		model.ProposalOptions{Payload: encode(t, pl)})

	_, _ = governance.Submit(ctx, s, p.ID(), "a", 1)
	_, _ = governance.Claim(ctx, s, p.ID(), "c", 2)
	_, err := governance.Accept(ctx, s, p.ID(), "c", 3)
	require.Error(t, err)
	require.True(t, lerrors.HasCode(err, lerrors.CodeFabricationAttempt))
}

func TestUnknownAncestor_AcceptsClean(t *testing.T) {
	s, ctx := newStore(t)
	defer s.Close()

	id := model.NewID()
	pl := governance.PayloadCreatePerson{NewID: id, UnknownAncestor: true}
	p := seedProposal(t, s, ctx, model.ProposalActionCreate, model.EntityKindPerson,
		model.ProposalOptions{Payload: encode(t, pl)})

	_, _ = governance.Submit(ctx, s, p.ID(), "a", 1)
	_, _ = governance.Claim(ctx, s, p.ID(), "c", 2)
	_, err := governance.Accept(ctx, s, p.ID(), "c", 3)
	require.NoError(t, err)

	rtx, _ := s.View(ctx)
	defer rtx.Close()
	got, err := rtx.GetPerson(id)
	require.NoError(t, err)
	require.True(t, got.IsUnknownAncestor())
}

func TestCycleDetection(t *testing.T) {
	s, ctx := newStore(t)
	defer s.Close()

	a := model.NewID()
	b := model.NewID()
	c := model.NewID()
	createPerson := func(id model.ID, name string) {
		pl := governance.PayloadCreatePerson{
			NewID: id,
			Names: []model.Name{{Text: name, Type: model.NameTypeGiven, Preferred: true}},
		}
		p := seedProposal(t, s, ctx, model.ProposalActionCreate, model.EntityKindPerson,
			model.ProposalOptions{Payload: encode(t, pl)})
		_, _ = governance.Submit(ctx, s, p.ID(), "a", 1)
		_, _ = governance.Claim(ctx, s, p.ID(), "c", 2)
		_, err := governance.Accept(ctx, s, p.ID(), "c", 3)
		require.NoError(t, err)
	}
	createRel := func(from, to model.ID) error {
		pl := governance.PayloadCreateRelationship{
			From: from, To: to, Type: model.RelTypeParentChild,
			Certainty:  model.CertaintyCertain,
			Continuity: model.NewContinuous(),
		}
		p := seedProposal(t, s, ctx, model.ProposalActionCreate, model.EntityKindRelationship,
			model.ProposalOptions{Payload: encode(t, pl)})
		_, _ = governance.Submit(ctx, s, p.ID(), "a", 1)
		_, _ = governance.Claim(ctx, s, p.ID(), "c", 2)
		_, err := governance.Accept(ctx, s, p.ID(), "c", 3)
		return err
	}
	createPerson(a, "A")
	createPerson(b, "B")
	createPerson(c, "C")

	// Build A -> B -> C
	require.NoError(t, createRel(a, b))
	require.NoError(t, createRel(b, c))

	// Adding C -> A would cycle
	err := createRel(c, a)
	require.Error(t, err)
	require.True(t, lerrors.HasCode(err, lerrors.CodeCycleDetected),
		"expected cycle, got: %v", err)
}

func TestMerge_RewritesRelationships(t *testing.T) {
	s, ctx := newStore(t)
	defer s.Close()

	// Direct seeding via store for a smaller test
	a := makePerson(t, "A")
	b := makePerson(t, "B") // duplicate of A
	parent := makePerson(t, "Parent")
	child := makePerson(t, "Child")

	relParentA := makeRel(t, parent.ID(), a.ID(), model.RelTypeParentChild, model.CertaintyCertain)
	relBChild := makeRel(t, b.ID(), child.ID(), model.RelTypeParentChild, model.CertaintyCertain)

	_, err := s.Update(ctx, func(tx store.WriteTx) error {
		for _, p := range []model.Person{a, b, parent, child} {
			if err := tx.PutPerson(p); err != nil {
				return err
			}
		}
		if err := tx.PutRelationship(relParentA); err != nil {
			return err
		}
		return tx.PutRelationship(relBChild)
	})
	require.NoError(t, err)

	// Merge: B -> A
	mp, err := model.NewProposal(model.NewID(), model.ProposalActionMerge, model.EntityKindPerson,
		model.ProposalOptions{TargetID: a.ID(), SecondaryID: b.ID(), Reason: "dup"})
	require.NoError(t, err)
	_, err = s.Update(ctx, func(tx store.WriteTx) error { return tx.PutProposal(mp) })
	require.NoError(t, err)

	_, _ = governance.Submit(ctx, s, mp.ID(), "actor", 1)
	_, _ = governance.Claim(ctx, s, mp.ID(), "actor", 2)
	_, err = governance.Accept(ctx, s, mp.ID(), "actor", 3)
	require.NoError(t, err)

	rtx, _ := s.View(ctx)
	defer rtx.Close()

	// B must be gone
	_, err = rtx.GetPerson(b.ID())
	require.True(t, lerrors.HasCode(err, lerrors.CodePersonNotFound))

	// The B->Child edge should now point A->Child
	rewritten, err := rtx.GetRelationship(relBChild.ID())
	require.NoError(t, err)
	require.Equal(t, a.ID(), rewritten.From())
	require.Equal(t, child.ID(), rewritten.To())
}

func TestSameAs_AnnotatesPerson(t *testing.T) {
	s, ctx := newStore(t)
	defer s.Close()

	a := makePerson(t, "A")
	b := makePerson(t, "B")
	_, err := s.Update(ctx, func(tx store.WriteTx) error {
		if err := tx.PutPerson(a); err != nil {
			return err
		}
		return tx.PutPerson(b)
	})
	require.NoError(t, err)

	mp, err := model.NewProposal(model.NewID(), model.ProposalActionSameAsLink, model.EntityKindPerson,
		model.ProposalOptions{TargetID: a.ID(), SecondaryID: b.ID()})
	require.NoError(t, err)
	_, err = s.Update(ctx, func(tx store.WriteTx) error { return tx.PutProposal(mp) })
	require.NoError(t, err)

	_, _ = governance.Submit(ctx, s, mp.ID(), "actor", 1)
	_, _ = governance.Claim(ctx, s, mp.ID(), "actor", 2)
	_, err = governance.Accept(ctx, s, mp.ID(), "actor", 3)
	require.NoError(t, err)

	rtx, _ := s.View(ctx)
	defer rtx.Close()
	got, err := rtx.GetPerson(a.ID())
	require.NoError(t, err)
	require.Contains(t, got.Notes(), "[same-as:"+b.ID().String()+"]")

	// B remains intact (non-destructive)
	_, err = rtx.GetPerson(b.ID())
	require.NoError(t, err)
}

func TestRetract_RefusesPersonWithIncidentRelationships(t *testing.T) {
	s, ctx := newStore(t)
	defer s.Close()

	a := makePerson(t, "A")
	b := makePerson(t, "B")
	r := makeRel(t, a.ID(), b.ID(), model.RelTypeParentChild, model.CertaintyCertain)

	_, err := s.Update(ctx, func(tx store.WriteTx) error {
		_ = tx.PutPerson(a)
		_ = tx.PutPerson(b)
		return tx.PutRelationship(r)
	})
	require.NoError(t, err)

	rp, err := model.NewProposal(model.NewID(), model.ProposalActionRetract, model.EntityKindPerson,
		model.ProposalOptions{TargetID: a.ID(), Reason: "delete"})
	require.NoError(t, err)
	_, err = s.Update(ctx, func(tx store.WriteTx) error { return tx.PutProposal(rp) })
	require.NoError(t, err)

	_, _ = governance.Submit(ctx, s, rp.ID(), "x", 1)
	_, _ = governance.Claim(ctx, s, rp.ID(), "x", 2)
	_, err = governance.Accept(ctx, s, rp.ID(), "x", 3)
	require.Error(t, err)
	require.True(t, lerrors.HasCode(err, lerrors.CodeInvalidArgument))
}

// ---- helpers ----

func makePerson(t *testing.T, name string) model.Person {
	t.Helper()
	n, err := model.NewName(name, "en", "Latn", model.NameTypeFull, true)
	require.NoError(t, err)
	p, err := model.NewPerson(model.NewID(), model.PersonOptions{Names: []model.Name{n}})
	require.NoError(t, err)
	return p
}

func makeRel(t *testing.T, from, to model.ID, typ model.RelationshipType, c model.Certainty) model.Relationship {
	t.Helper()
	r, err := model.NewRelationship(model.NewID(), from, to, typ, c, model.NewContinuous(), model.RelationshipOptions{})
	require.NoError(t, err)
	return r
}
