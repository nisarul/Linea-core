// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import "fmt"

// ProposalState is one of the five states in the proposal
// lifecycle defined by CCGGS §8.3:
//
//	Draft → Submitted → UnderReview → (Accepted | Rejected | Withdrawn)
type ProposalState uint8

const (
	// ProposalDraft — author working, not yet visible to curators.
	ProposalDraft ProposalState = 1
	// ProposalSubmitted — visible, awaiting review.
	ProposalSubmitted ProposalState = 2
	// ProposalUnderReview — claimed by a curator.
	ProposalUnderReview ProposalState = 3
	// ProposalAccepted — applied to the graph; immutable thereafter.
	ProposalAccepted ProposalState = 4
	// ProposalRejected — not applied; rationale required; immutable.
	ProposalRejected ProposalState = 5
	// ProposalWithdrawn — pulled by author before acceptance; immutable.
	ProposalWithdrawn ProposalState = 6
)

// IsValid reports whether s is one of the six states.
func (s ProposalState) IsValid() bool {
	return s >= ProposalDraft && s <= ProposalWithdrawn
}

// IsTerminal reports whether s is a terminal state (Accepted,
// Rejected, or Withdrawn). Terminal states MUST NOT transition.
func (s ProposalState) IsTerminal() bool {
	return s == ProposalAccepted || s == ProposalRejected || s == ProposalWithdrawn
}

// String returns the spec-defined name of the state.
func (s ProposalState) String() string {
	switch s {
	case ProposalDraft:
		return "Draft"
	case ProposalSubmitted:
		return "Submitted"
	case ProposalUnderReview:
		return "UnderReview"
	case ProposalAccepted:
		return "Accepted"
	case ProposalRejected:
		return "Rejected"
	case ProposalWithdrawn:
		return "Withdrawn"
	default:
		return fmt.Sprintf("ProposalState(%d)", uint8(s))
	}
}

// ProposalAction is the kind of mutation a Proposal proposes.
type ProposalAction uint8

const (
	// ProposalActionCreate creates a new entity.
	ProposalActionCreate ProposalAction = 1
	// ProposalActionUpdate updates an existing entity.
	ProposalActionUpdate ProposalAction = 2
	// ProposalActionRetract retracts (logically deletes) an entity.
	// Retraction preserves audit history; it does not erase data.
	ProposalActionRetract ProposalAction = 3
	// ProposalActionMerge merges two persons into one canonical
	// person, per CCGGS §11.3.
	ProposalActionMerge ProposalAction = 4
	// ProposalActionSameAsLink records a non-destructive same-as
	// link between two persons (CCGGS §11.3 alternative to merge).
	ProposalActionSameAsLink ProposalAction = 5
)

// IsValid reports whether a is one of the allowed actions.
func (a ProposalAction) IsValid() bool {
	return a >= ProposalActionCreate && a <= ProposalActionSameAsLink
}

// String returns a human-readable name for the action.
func (a ProposalAction) String() string {
	switch a {
	case ProposalActionCreate:
		return "Create"
	case ProposalActionUpdate:
		return "Update"
	case ProposalActionRetract:
		return "Retract"
	case ProposalActionMerge:
		return "Merge"
	case ProposalActionSameAsLink:
		return "SameAsLink"
	default:
		return fmt.Sprintf("ProposalAction(%d)", uint8(a))
	}
}

// EntityKind identifies which kind of entity a Proposal targets.
type EntityKind uint8

const (
	// EntityKindPerson targets a Person.
	EntityKindPerson EntityKind = 1
	// EntityKindRelationship targets a Relationship.
	EntityKindRelationship EntityKind = 2
	// EntityKindSource targets a Source.
	EntityKindSource EntityKind = 3
)

// IsValid reports whether k is one of the allowed kinds.
func (k EntityKind) IsValid() bool {
	return k >= EntityKindPerson && k <= EntityKindSource
}

// String returns a human-readable name for the kind.
func (k EntityKind) String() string {
	switch k {
	case EntityKindPerson:
		return "Person"
	case EntityKindRelationship:
		return "Relationship"
	case EntityKindSource:
		return "Source"
	default:
		return fmt.Sprintf("EntityKind(%d)", uint8(k))
	}
}

// ProposalTransition records a single state transition in a
// proposal's audit history.
type ProposalTransition struct {
	From      ProposalState
	To        ProposalState
	Actor     string // opaque actor identifier (user id, system, etc.)
	Timestamp int64  // unix epoch seconds
	Reason    string // optional rationale, REQUIRED for Rejected
}

// Proposal is the only path through which the genealogical graph
// may be mutated (CCGGS §8). Every accepted proposal produces a
// new graph version; rejected and withdrawn proposals are also
// preserved in audit history.
//
// The Proposal value records the *intent*; the actual application
// of changes lives in package governance.
type Proposal struct {
	id          ID
	state       ProposalState
	action      ProposalAction
	entityKind  EntityKind
	targetID    ID    // empty for Create with new ID; set for others
	secondaryID ID    // for Merge/SameAsLink: the other person
	payload     []byte // opaque encoded change descriptor
	reason      string
	sources     []ID
	author      string
	createdAt   int64
	history     []ProposalTransition
}

// ProposalOptions bundles fields for NewProposal.
type ProposalOptions struct {
	TargetID    ID
	SecondaryID ID
	Payload     []byte
	Reason      string
	Sources     []ID
	Author      string
	CreatedAt   int64
}

// NewProposal constructs a Proposal in Draft state with validated
// inputs. The caller is responsible for transitioning the
// proposal through later states via package governance.
func NewProposal(
	id ID,
	action ProposalAction,
	entityKind EntityKind,
	opts ProposalOptions,
) (Proposal, error) {
	if id.IsZero() {
		return Proposal{}, fmt.Errorf("model: Proposal ID is required")
	}
	if !action.IsValid() {
		return Proposal{}, fmt.Errorf("model: Proposal %s has invalid action %v", id, action)
	}
	if !entityKind.IsValid() {
		return Proposal{}, fmt.Errorf("model: Proposal %s has invalid entity kind %v", id, entityKind)
	}
	switch action {
	case ProposalActionUpdate, ProposalActionRetract:
		if opts.TargetID.IsZero() {
			return Proposal{}, fmt.Errorf(
				"model: Proposal %s with action %s requires TargetID", id, action,
			)
		}
	case ProposalActionMerge, ProposalActionSameAsLink:
		if entityKind != EntityKindPerson {
			return Proposal{}, fmt.Errorf(
				"model: Proposal %s action %s requires EntityKindPerson", id, action,
			)
		}
		if opts.TargetID.IsZero() || opts.SecondaryID.IsZero() {
			return Proposal{}, fmt.Errorf(
				"model: Proposal %s action %s requires TargetID and SecondaryID", id, action,
			)
		}
		if opts.TargetID == opts.SecondaryID {
			return Proposal{}, fmt.Errorf(
				"model: Proposal %s action %s: TargetID and SecondaryID must differ", id, action,
			)
		}
	}
	return Proposal{
		id:          id,
		state:       ProposalDraft,
		action:      action,
		entityKind:  entityKind,
		targetID:    opts.TargetID,
		secondaryID: opts.SecondaryID,
		payload:     append([]byte(nil), opts.Payload...),
		reason:      opts.Reason,
		sources:     append([]ID(nil), opts.Sources...),
		author:      opts.Author,
		createdAt:   opts.CreatedAt,
	}, nil
}

// ID returns the proposal's stable identifier.
func (p Proposal) ID() ID { return p.id }

// State returns the current proposal state.
func (p Proposal) State() ProposalState { return p.state }

// Action returns the proposed action.
func (p Proposal) Action() ProposalAction { return p.action }

// EntityKind returns the kind of entity this proposal targets.
func (p Proposal) EntityKind() EntityKind { return p.entityKind }

// TargetID returns the primary target entity ID (or zero for
// Create-with-new-ID).
func (p Proposal) TargetID() ID { return p.targetID }

// SecondaryID returns the secondary target ID (used by Merge and
// SameAsLink).
func (p Proposal) SecondaryID() ID { return p.secondaryID }

// Payload returns the opaque change descriptor.
func (p Proposal) Payload() []byte {
	out := make([]byte, len(p.payload))
	copy(out, p.payload)
	return out
}

// Reason returns the proposal rationale.
func (p Proposal) Reason() string { return p.reason }

// Sources returns supporting Source IDs.
func (p Proposal) Sources() []ID {
	out := make([]ID, len(p.sources))
	copy(out, p.sources)
	return out
}

// Author returns the opaque author identifier.
func (p Proposal) Author() string { return p.author }

// CreatedAt returns the creation timestamp (unix seconds).
func (p Proposal) CreatedAt() int64 { return p.createdAt }

// History returns a copy of the audit history of state transitions.
func (p Proposal) History() []ProposalTransition {
	out := make([]ProposalTransition, len(p.history))
	copy(out, p.history)
	return out
}

// withState returns a copy of p with state and an appended
// transition. It is unexported; the legal transition machine is
// enforced by package governance.
func (p Proposal) withState(s ProposalState, tr ProposalTransition) Proposal {
	cp := p
	cp.state = s
	cp.history = append(append([]ProposalTransition(nil), p.history...), tr)
	return cp
}

// WithStateUnchecked produces a copy in the supplied state with
// the supplied audit transition appended.
//
// This bypasses transition validation and is intended for use
// ONLY by package governance, which performs its own validation,
// and by storage adapters reconstructing persisted proposals.
// Application code MUST NOT call this directly.
func (p Proposal) WithStateUnchecked(s ProposalState, tr ProposalTransition) Proposal {
	return p.withState(s, tr)
}

// String renders the proposal for diagnostics.
func (p Proposal) String() string {
	return fmt.Sprintf("Proposal<%s,%s,%s,%s>", p.id, p.state, p.action, p.entityKind)
}
