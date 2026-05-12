// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCertainty_RankAndOrdering(t *testing.T) {
	assert.Equal(t, 1, CertaintyUncertain.Rank())
	assert.Equal(t, 2, CertaintyProbable.Rank())
	assert.Equal(t, 3, CertaintyCertain.Rank())

	assert.Equal(t, "Certain", CertaintyCertain.String())
	assert.Equal(t, "Probable", CertaintyProbable.String())
	assert.Equal(t, "Uncertain", CertaintyUncertain.String())

	assert.True(t, CertaintyCertain.IsValid())
	assert.False(t, Certainty(0).IsValid())
	assert.False(t, Certainty(99).IsValid())
}

func TestCertainty_WeakestLink(t *testing.T) {
	cases := []struct {
		a, b, want Certainty
	}{
		{CertaintyCertain, CertaintyProbable, CertaintyProbable},
		{CertaintyProbable, CertaintyUncertain, CertaintyUncertain},
		{CertaintyCertain, CertaintyUncertain, CertaintyUncertain},
		{CertaintyCertain, CertaintyCertain, CertaintyCertain},
		{CertaintyUncertain, CertaintyUncertain, CertaintyUncertain},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, c.a.Min(c.b), "Min(%v,%v)", c.a, c.b)
		assert.Equal(t, c.want, c.b.Min(c.a), "commutative Min(%v,%v)", c.b, c.a)
	}
}

func TestCertainty_Combine_AssociativeAndCommutative(t *testing.T) {
	all := []Certainty{CertaintyCertain, CertaintyProbable, CertaintyUncertain}
	// Property: combining is associative and commutative; the
	// result equals the minimum rank of the inputs.
	for _, a := range all {
		for _, b := range all {
			for _, c := range all {
				lhs := CombineCertainties(a, b, c)
				rhs := CombineCertainties(c, b, a)
				assert.Equal(t, lhs, rhs, "commutativity broken for %v,%v,%v", a, b, c)
				lhs2 := a.Min(b).Min(c)
				assert.Equal(t, lhs, lhs2, "associativity broken for %v,%v,%v", a, b, c)
			}
		}
	}
}

func TestCertainty_Combine_PanicsOnEmpty(t *testing.T) {
	assert.Panics(t, func() { _ = CombineCertainties() })
}

func TestContinuity_Validation(t *testing.T) {
	require.True(t, NewContinuous().IsValid())
	require.True(t, NewContinuous().State == ContinuityContinuous)
	require.False(t, NewContinuous().IsGapped())

	g, err := KnownGap(3)
	require.NoError(t, err)
	c := NewGapped(g)
	require.True(t, c.IsValid())
	require.True(t, c.IsGapped())

	_, err = KnownGap(0)
	require.Error(t, err)
	_, err = KnownGap(-5)
	require.Error(t, err)

	// Unknown-size gap is valid
	require.True(t, NewGapped(UnknownGap()).IsValid())

	// Manually-constructed broken value is invalid
	broken := Continuity{State: ContinuityGapped, Gap: GapGenerations{KnownSize: true, Size: 0}}
	require.False(t, broken.IsValid())

	bad := Continuity{State: ContinuityState(99)}
	require.False(t, bad.IsValid())
}

func TestGender_ParseAndCore(t *testing.T) {
	g, err := ParseGender("", false)
	require.NoError(t, err)
	require.True(t, g.IsUnset())

	g, err = ParseGender("Female", false)
	require.NoError(t, err)
	require.Equal(t, GenderFemale, g)
	require.True(t, g.IsCore())

	_, err = ParseGender("nonbinary", false)
	require.Error(t, err)

	g, err = ParseGender("nonbinary", true)
	require.NoError(t, err)
	require.False(t, g.IsCore())
	require.Equal(t, Gender("nonbinary"), g)
}

func TestTimeRange_Validation(t *testing.T) {
	_, err := NewTimeRange(KnownYearBound(2000), KnownYearBound(1990), CalendarGregorianProleptic, false)
	require.Error(t, err, "earliest after latest must fail")

	tr, err := NewTimeRange(KnownYearBound(1900), KnownYearBound(1950), "", false)
	require.NoError(t, err)
	require.Equal(t, CalendarGregorianProleptic, tr.Calendar)

	tr, err = NewTimeRange(UnknownYear(), KnownYearBound(1500), CalendarJulian, true)
	require.NoError(t, err)
	require.True(t, tr.Circa)
	require.Equal(t, CalendarJulian, tr.Calendar)

	_, err = NewTimeRange(UnknownYear(), UnknownYear(), Calendar(""), false)
	require.NoError(t, err)
}

func TestPerson_Construction(t *testing.T) {
	id := NewID()
	require.False(t, id.IsZero())

	n, err := NewName("Alice", "en", "Latn", NameTypeGiven, true)
	require.NoError(t, err)

	p, err := NewPerson(id, PersonOptions{Names: []Name{n}, Gender: GenderFemale})
	require.NoError(t, err)
	require.Equal(t, id, p.ID())
	require.Equal(t, "Alice", p.PreferredName().Text)
	require.False(t, p.IsUnknownAncestor())

	// Required: ID
	_, err = NewPerson("", PersonOptions{Names: []Name{n}})
	require.Error(t, err)

	// Required: at least one name for a normal Person
	_, err = NewPerson(id, PersonOptions{})
	require.Error(t, err)
}

func TestPerson_UnknownAncestor_NoFabrication(t *testing.T) {
	id := NewID()
	p, err := NewUnknownAncestor(id)
	require.NoError(t, err)
	require.True(t, p.IsUnknownAncestor())
	require.Empty(t, p.Names())
	require.True(t, p.Gender().IsUnset())
	require.True(t, p.Birth().IsZero())
	require.True(t, p.Death().IsZero())
}

func TestRelationship_Validation(t *testing.T) {
	from, to := NewID(), NewID()

	r, err := NewRelationship(NewID(), from, to, RelTypeParentChild,
		CertaintyCertain, NewContinuous(), RelationshipOptions{})
	require.NoError(t, err)
	require.Equal(t, RelTypeParentChild, r.Type())
	require.Equal(t, 1, r.GapWeight())

	// Self-loop forbidden
	_, err = NewRelationship(NewID(), from, from, RelTypeParentChild,
		CertaintyCertain, NewContinuous(), RelationshipOptions{})
	require.Error(t, err)

	// Marriage cannot be Gapped
	gap, _ := KnownGap(2)
	_, err = NewRelationship(NewID(), from, to, RelTypeMarriage,
		CertaintyCertain, NewGapped(gap), RelationshipOptions{})
	require.Error(t, err)

	// Marriage weight is 0
	m, err := NewRelationship(NewID(), from, to, RelTypeMarriage,
		CertaintyCertain, NewContinuous(), RelationshipOptions{})
	require.NoError(t, err)
	require.Equal(t, 0, m.GapWeight())

	// Gapped ParentChild with known size 3 → weight 4
	pc, err := NewRelationship(NewID(), from, to, RelTypeParentChild,
		CertaintyProbable, NewGapped(gap), RelationshipOptions{})
	require.NoError(t, err)
	require.Equal(t, 3, pc.GapWeight())

	// Gapped ParentChild with unknown size → sentinel weight
	pcU, err := NewRelationship(NewID(), from, to, RelTypeParentChild,
		CertaintyUncertain, NewGapped(UnknownGap()), RelationshipOptions{})
	require.NoError(t, err)
	require.Greater(t, pcU.GapWeight(), 1000)

	// Invalid certainty
	_, err = NewRelationship(NewID(), from, to, RelTypeParentChild,
		Certainty(0), NewContinuous(), RelationshipOptions{})
	require.Error(t, err)

	// Invalid type
	_, err = NewRelationship(NewID(), from, to, RelationshipType(0),
		CertaintyCertain, NewContinuous(), RelationshipOptions{})
	require.Error(t, err)

	// Required ID
	_, err = NewRelationship("", from, to, RelTypeParentChild,
		CertaintyCertain, NewContinuous(), RelationshipOptions{})
	require.Error(t, err)
}

func TestSource_Validation(t *testing.T) {
	_, err := NewSource("", SourceTypePrimary, "cite", SourceOptions{})
	require.Error(t, err)

	_, err = NewSource(NewID(), SourceTypePrimary, "   ", SourceOptions{})
	require.Error(t, err)

	s, err := NewSource(NewID(), "", "anonymous oral tradition", SourceOptions{})
	require.NoError(t, err)
	require.Equal(t, SourceTypeOther, s.Type())
}

func TestProposal_DraftAndValidation(t *testing.T) {
	p, err := NewProposal(NewID(), ProposalActionCreate, EntityKindPerson,
		ProposalOptions{Author: "alice", CreatedAt: 1, Reason: "new"})
	require.NoError(t, err)
	require.Equal(t, ProposalDraft, p.State())
	require.False(t, ProposalDraft.IsTerminal())
	require.True(t, ProposalAccepted.IsTerminal())
	require.True(t, ProposalRejected.IsTerminal())
	require.True(t, ProposalWithdrawn.IsTerminal())

	// Update requires TargetID
	_, err = NewProposal(NewID(), ProposalActionUpdate, EntityKindPerson,
		ProposalOptions{})
	require.Error(t, err)

	// Merge requires both IDs and EntityKindPerson
	_, err = NewProposal(NewID(), ProposalActionMerge, EntityKindRelationship,
		ProposalOptions{TargetID: NewID(), SecondaryID: NewID()})
	require.Error(t, err)

	tid := NewID()
	_, err = NewProposal(NewID(), ProposalActionMerge, EntityKindPerson,
		ProposalOptions{TargetID: tid, SecondaryID: tid})
	require.Error(t, err, "merge with same IDs must fail")

	_, err = NewProposal(NewID(), ProposalActionMerge, EntityKindPerson,
		ProposalOptions{TargetID: NewID(), SecondaryID: NewID()})
	require.NoError(t, err)
}
