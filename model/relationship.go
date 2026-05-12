// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import "fmt"

// RelationshipType enumerates the only allowed primitive
// relationship types per CCGGS §5.1.
type RelationshipType uint8

const (
	// RelTypeParentChild is a biological Parent → Child edge.
	// The "from" person is the parent; the "to" person is the child.
	RelTypeParentChild RelationshipType = 1
	// RelTypeMarriage is a Marriage edge between two persons.
	// Marriage is the only affinal primitive; it is undirected
	// but stored with a canonical (from, to) ordering.
	RelTypeMarriage RelationshipType = 2
)

// IsValid reports whether t is one of the two allowed types.
func (t RelationshipType) IsValid() bool {
	return t == RelTypeParentChild || t == RelTypeMarriage
}

// String returns the spec name of the relationship type.
func (t RelationshipType) String() string {
	switch t {
	case RelTypeParentChild:
		return "ParentChild"
	case RelTypeMarriage:
		return "Marriage"
	default:
		return fmt.Sprintf("RelationshipType(%d)", uint8(t))
	}
}

// IsDirected reports whether the relationship type carries
// directionality (Parent → Child does; Marriage does not).
func (t RelationshipType) IsDirected() bool {
	return t == RelTypeParentChild
}

// Relationship is a typed edge between two Persons (CCGGS §5).
//
// All fields are validated at construction time via the
// NewRelationship constructor. Relationships are immutable.
type Relationship struct {
	id         ID
	from       ID
	to         ID
	relType    RelationshipType
	certainty  Certainty
	continuity Continuity
	timeRange  TimeRange // optional; may be zero
	notes      string
	sources    []ID // Source IDs attached to this edge
}

// RelationshipOptions carries optional fields for NewRelationship.
type RelationshipOptions struct {
	TimeRange TimeRange
	Notes     string
	Sources   []ID
}

// NewRelationship constructs and validates a Relationship.
//
// It enforces:
//   - relType must be one of the allowed primitive types
//   - from and to must be non-zero and not equal
//   - certainty must be one of the three valid values
//   - continuity must be internally consistent
//   - if continuity is Gapped, the relationship type must be
//     RelTypeParentChild (Marriage cannot be "gapped")
func NewRelationship(
	id, from, to ID,
	relType RelationshipType,
	cert Certainty,
	cont Continuity,
	opts RelationshipOptions,
) (Relationship, error) {
	if id.IsZero() {
		return Relationship{}, fmt.Errorf("model: Relationship ID is required")
	}
	if from.IsZero() || to.IsZero() {
		return Relationship{}, fmt.Errorf("model: Relationship %s requires both endpoints", id)
	}
	if from == to {
		return Relationship{}, fmt.Errorf("model: Relationship %s endpoints must differ", id)
	}
	if !relType.IsValid() {
		return Relationship{}, fmt.Errorf("model: Relationship %s has invalid type %v", id, relType)
	}
	if !cert.IsValid() {
		return Relationship{}, fmt.Errorf("model: Relationship %s has invalid certainty %v", id, cert)
	}
	if !cont.IsValid() {
		return Relationship{}, fmt.Errorf("model: Relationship %s has invalid continuity %v", id, cont)
	}
	if cont.IsGapped() && relType != RelTypeParentChild {
		return Relationship{}, fmt.Errorf(
			"model: Relationship %s: Gapped continuity is only valid on ParentChild edges",
			id,
		)
	}
	return Relationship{
		id:         id,
		from:       from,
		to:         to,
		relType:    relType,
		certainty:  cert,
		continuity: cont,
		timeRange:  opts.TimeRange,
		notes:      opts.Notes,
		sources:    append([]ID(nil), opts.Sources...),
	}, nil
}

// ID returns the relationship's stable identifier.
func (r Relationship) ID() ID { return r.id }

// From returns the source endpoint (parent for ParentChild,
// canonical first endpoint for Marriage).
func (r Relationship) From() ID { return r.from }

// To returns the target endpoint (child for ParentChild,
// canonical second endpoint for Marriage).
func (r Relationship) To() ID { return r.to }

// Type returns the relationship type.
func (r Relationship) Type() RelationshipType { return r.relType }

// Certainty returns the certainty of this edge.
func (r Relationship) Certainty() Certainty { return r.certainty }

// Continuity returns the continuity (state + gap size) of this edge.
func (r Relationship) Continuity() Continuity { return r.continuity }

// GapWeight returns the number of generations this edge accounts
// for in path-length calculations:
//
//   - Continuous: 1
//   - Gapped with KnownGap(n): n + 1   (the gap itself + the edge)
//   - Gapped with UnknownGap: a large sentinel weight so that
//     paths with unknown-size gaps are ranked worst.
//
// Marriage edges always weigh 0 (they connect lineages without
// adding generations); the engine treats them as cross-lineage links.
func (r Relationship) GapWeight() int {
	if r.relType == RelTypeMarriage {
		return 0
	}
	if !r.continuity.IsGapped() {
		return 1
	}
	if r.continuity.Gap.KnownSize {
		return r.continuity.Gap.Size + 1
	}
	return unknownGapSentinel
}

// unknownGapSentinel ranks unknown-size gaps worse than any
// realistic known gap. Chosen large enough to dominate any
// reasonable summation but small enough to avoid overflow.
const unknownGapSentinel = 1_000_000

// TimeRange returns the optional time range of the relationship.
func (r Relationship) TimeRange() TimeRange { return r.timeRange }

// Notes returns free-form notes attached to the edge.
func (r Relationship) Notes() string { return r.notes }

// Sources returns a copy of the Source IDs attached to this edge.
func (r Relationship) Sources() []ID {
	out := make([]ID, len(r.sources))
	copy(out, r.sources)
	return out
}

// String renders the relationship for diagnostics.
func (r Relationship) String() string {
	return fmt.Sprintf(
		"Rel<%s, %s: %s -> %s, %s, %s>",
		r.id, r.relType, r.from, r.to, r.certainty, r.continuity,
	)
}
