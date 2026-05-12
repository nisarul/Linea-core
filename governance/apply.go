// SPDX-License-Identifier: AGPL-3.0-or-later

package governance

import (
	"context"
	"encoding/json"
	"fmt"

	lerrors "github.com/nisarul/Linea-core/errors"
	"github.com/nisarul/Linea-core/model"
	"github.com/nisarul/Linea-core/store"
)

// PayloadCreatePerson is the payload schema for an
// EntityKindPerson + ProposalActionCreate proposal.
//
// The accept path constructs a Person via model.NewPerson, so the
// resulting graph value is fully validated.
type PayloadCreatePerson struct {
	NewID            model.ID                   `json:"id,omitempty"`
	Names            []model.Name               `json:"names,omitempty"`
	Gender           model.Gender               `json:"gender,omitempty"`
	Birth            model.TimeRange            `json:"birth,omitempty"`
	Death            model.TimeRange            `json:"death,omitempty"`
	Notes            string                     `json:"notes,omitempty"`
	UnknownAncestor  bool                       `json:"unknown,omitempty"`
}

// PayloadCreateRelationship targets EntityKindRelationship + Create.
type PayloadCreateRelationship struct {
	NewID      model.ID               `json:"id,omitempty"`
	From       model.ID               `json:"from"`
	To         model.ID               `json:"to"`
	Type       model.RelationshipType `json:"type"`
	Certainty  model.Certainty        `json:"certainty"`
	Continuity model.Continuity       `json:"continuity"`
	TimeRange  model.TimeRange        `json:"timeRange,omitempty"`
	Notes      string                 `json:"notes,omitempty"`
	Sources    []model.ID             `json:"sources,omitempty"`
}

// PayloadCreateSource targets EntityKindSource + Create.
type PayloadCreateSource struct {
	NewID    model.ID         `json:"id,omitempty"`
	Type     model.SourceType `json:"type,omitempty"`
	Citation string           `json:"citation"`
	Author   string           `json:"author,omitempty"`
	Title    string           `json:"title,omitempty"`
	Date     string           `json:"date,omitempty"`
	Locator  string           `json:"locator,omitempty"`
	Notes    string           `json:"notes,omitempty"`
}

// PayloadRetract targets any kind + Retract. Retraction logically
// removes the entity. For relationships, only that edge is
// removed; for persons, the caller is responsible for retracting
// incident relationships first (Apply enforces this).
type PayloadRetract struct{}

// Submit transitions a Draft proposal to Submitted and persists
// the change atomically.
func Submit(ctx context.Context, s store.Store, propID model.ID, actor string, ts int64) (model.Proposal, error) {
	return transitionInStore(ctx, s, propID, model.ProposalSubmitted, actor, ts, "")
}

// Claim transitions Submitted → UnderReview.
func Claim(ctx context.Context, s store.Store, propID model.ID, actor string, ts int64) (model.Proposal, error) {
	return transitionInStore(ctx, s, propID, model.ProposalUnderReview, actor, ts, "")
}

// Withdraw moves any non-terminal proposal to Withdrawn.
func Withdraw(ctx context.Context, s store.Store, propID model.ID, actor string, ts int64, reason string) (model.Proposal, error) {
	return transitionInStore(ctx, s, propID, model.ProposalWithdrawn, actor, ts, reason)
}

// Reject moves UnderReview → Rejected. Reason is required.
func Reject(ctx context.Context, s store.Store, propID model.ID, actor string, ts int64, reason string) (model.Proposal, error) {
	return transitionInStore(ctx, s, propID, model.ProposalRejected, actor, ts, reason)
}

// Accept moves UnderReview → Accepted, applies the proposed
// mutation to the graph, and commits both atomically.
//
// On Apply failure, the entire transaction is rolled back and
// the proposal stays in UnderReview.
func Accept(ctx context.Context, s store.Store, propID model.ID, actor string, ts int64) (model.Proposal, error) {
	var out model.Proposal
	_, err := s.Update(ctx, func(tx store.WriteTx) error {
		p, err := tx.GetProposal(propID)
		if err != nil {
			return err
		}
		updated, err := Transition(p, model.ProposalAccepted, actor, ts, "")
		if err != nil {
			return err
		}
		if err := apply(tx, updated); err != nil {
			return err
		}
		if err := tx.PutProposal(updated); err != nil {
			return err
		}
		out = updated
		return nil
	})
	if err != nil {
		return model.Proposal{}, err
	}
	return out, nil
}

// transitionInStore is the shared body for non-Accept transitions
// (Submit / Claim / Withdraw / Reject), which never mutate graph
// entities — only the proposal record.
func transitionInStore(
	ctx context.Context, s store.Store,
	propID model.ID, to model.ProposalState,
	actor string, ts int64, reason string,
) (model.Proposal, error) {
	var out model.Proposal
	_, err := s.Update(ctx, func(tx store.WriteTx) error {
		p, err := tx.GetProposal(propID)
		if err != nil {
			return err
		}
		updated, err := Transition(p, to, actor, ts, reason)
		if err != nil {
			return err
		}
		if err := tx.PutProposal(updated); err != nil {
			return err
		}
		out = updated
		return nil
	})
	if err != nil {
		return model.Proposal{}, err
	}
	return out, nil
}

// apply executes the entity-level mutation described by the
// proposal payload. Called only when transitioning to Accepted.
func apply(tx store.WriteTx, p model.Proposal) error {
	switch p.Action() {
	case model.ProposalActionCreate:
		return applyCreate(tx, p)
	case model.ProposalActionUpdate:
		return applyUpdate(tx, p)
	case model.ProposalActionRetract:
		return applyRetract(tx, p)
	case model.ProposalActionMerge:
		return applyMerge(tx, p)
	case model.ProposalActionSameAsLink:
		return applySameAs(tx, p)
	}
	return lerrors.New(lerrors.CodeInvalidArgument,
		fmt.Sprintf("proposal %s: unsupported action %s", p.ID(), p.Action()))
}

func applyCreate(tx store.WriteTx, p model.Proposal) error {
	switch p.EntityKind() {
	case model.EntityKindPerson:
		var pl PayloadCreatePerson
		if err := json.Unmarshal(p.Payload(), &pl); err != nil {
			return lerrors.Wrap(lerrors.CodeInvalidArgument,
				fmt.Sprintf("proposal %s: bad person payload", p.ID()), err)
		}
		id := pl.NewID
		if id.IsZero() {
			id = model.NewID()
		}
		if pl.UnknownAncestor {
			// CCGGS §5.3: unknown placeholders carry no fabricated
			// attributes. Reject any payload that tries.
			if len(pl.Names) > 0 || pl.Gender != model.GenderUnset ||
				!pl.Birth.IsZero() || !pl.Death.IsZero() || pl.Notes != "" {
				return lerrors.New(lerrors.CodeFabricationAttempt,
					fmt.Sprintf("proposal %s: unknown-ancestor payload contains substantive attributes", p.ID()))
			}
			ua, err := model.NewUnknownAncestor(id)
			if err != nil {
				return err
			}
			return tx.PutPerson(ua)
		}
		person, err := model.NewPerson(id, model.PersonOptions{
			Names:  pl.Names,
			Gender: pl.Gender,
			Birth:  pl.Birth,
			Death:  pl.Death,
			Notes:  pl.Notes,
		})
		if err != nil {
			return err
		}
		return tx.PutPerson(person)

	case model.EntityKindRelationship:
		var pl PayloadCreateRelationship
		if err := json.Unmarshal(p.Payload(), &pl); err != nil {
			return lerrors.Wrap(lerrors.CodeInvalidArgument,
				fmt.Sprintf("proposal %s: bad relationship payload", p.ID()), err)
		}
		id := pl.NewID
		if id.IsZero() {
			id = model.NewID()
		}
		rel, err := model.NewRelationship(id, pl.From, pl.To,
			pl.Type, pl.Certainty, pl.Continuity, model.RelationshipOptions{
				TimeRange: pl.TimeRange,
				Notes:     pl.Notes,
				Sources:   pl.Sources,
			})
		if err != nil {
			return err
		}
		// Cycle prevention: refuse to create a parent-child edge
		// whose child is already an ancestor of the proposed parent.
		if rel.Type() == model.RelTypeParentChild {
			if cyc, err := wouldCreateCycle(tx, rel.From(), rel.To()); err != nil {
				return err
			} else if cyc {
				return lerrors.New(lerrors.CodeCycleDetected,
					fmt.Sprintf("proposal %s: parent-child edge %s -> %s would create a cycle",
						p.ID(), rel.From(), rel.To()))
			}
		}
		return tx.PutRelationship(rel)

	case model.EntityKindSource:
		var pl PayloadCreateSource
		if err := json.Unmarshal(p.Payload(), &pl); err != nil {
			return lerrors.Wrap(lerrors.CodeInvalidArgument,
				fmt.Sprintf("proposal %s: bad source payload", p.ID()), err)
		}
		id := pl.NewID
		if id.IsZero() {
			id = model.NewID()
		}
		src, err := model.NewSource(id, pl.Type, pl.Citation, model.SourceOptions{
			Author:  pl.Author,
			Title:   pl.Title,
			Date:    pl.Date,
			Locator: pl.Locator,
			Notes:   pl.Notes,
		})
		if err != nil {
			return err
		}
		return tx.PutSource(src)
	}
	return lerrors.New(lerrors.CodeInvalidArgument,
		fmt.Sprintf("proposal %s: unsupported entity kind %s for Create", p.ID(), p.EntityKind()))
}

// applyUpdate is intentionally not implemented in v0.1: the
// Update payload schema for partial mutations is left for a
// follow-up. Returning a typed error keeps callers explicit.
func applyUpdate(_ store.WriteTx, p model.Proposal) error {
	return lerrors.New(lerrors.CodeInvalidArgument,
		fmt.Sprintf("proposal %s: Update action not yet implemented", p.ID()))
}

func applyRetract(tx store.WriteTx, p model.Proposal) error {
	if p.TargetID().IsZero() {
		return lerrors.New(lerrors.CodeInvalidArgument,
			fmt.Sprintf("proposal %s: Retract requires TargetID", p.ID()))
	}
	switch p.EntityKind() {
	case model.EntityKindPerson:
		// Refuse to delete a person who still has incident
		// relationships — caller must retract those first.
		incident := false
		_ = tx.IterateChildren(p.TargetID(), func(model.Relationship) bool { incident = true; return false })
		if !incident {
			_ = tx.IterateParents(p.TargetID(), func(model.Relationship) bool { incident = true; return false })
		}
		if !incident {
			_ = tx.IterateMarriages(p.TargetID(), func(model.Relationship) bool { incident = true; return false })
		}
		if incident {
			return lerrors.New(lerrors.CodeInvalidArgument,
				fmt.Sprintf("proposal %s: person %s still has incident relationships", p.ID(), p.TargetID()))
		}
		return tx.DeletePerson(p.TargetID())
	case model.EntityKindRelationship:
		return tx.DeleteRelationship(p.TargetID())
	case model.EntityKindSource:
		return tx.DeleteSource(p.TargetID())
	}
	return lerrors.New(lerrors.CodeInvalidArgument,
		fmt.Sprintf("proposal %s: unsupported entity kind %s for Retract", p.ID(), p.EntityKind()))
}

// applyMerge implements CCGGS §11.3 merge: the Secondary person
// is folded into the Target. Every relationship referencing the
// Secondary is rewritten to the Target; the Secondary is then
// removed. Self-loops produced by the rewrite are dropped.
func applyMerge(tx store.WriteTx, p model.Proposal) error {
	target := p.TargetID()
	secondary := p.SecondaryID()
	if target.IsZero() || secondary.IsZero() || target == secondary {
		return lerrors.New(lerrors.CodeInvalidArgument,
			fmt.Sprintf("proposal %s: Merge requires distinct TargetID and SecondaryID", p.ID()))
	}
	if _, err := tx.GetPerson(target); err != nil {
		return err
	}
	if _, err := tx.GetPerson(secondary); err != nil {
		return err
	}
	// Collect all relationships touching `secondary`.
	var touching []model.Relationship
	if err := tx.IterateRelationships(func(r model.Relationship) bool {
		if r.From() == secondary || r.To() == secondary {
			touching = append(touching, r)
		}
		return true
	}); err != nil {
		return err
	}
	for _, r := range touching {
		from, to := r.From(), r.To()
		if from == secondary {
			from = target
		}
		if to == secondary {
			to = target
		}
		if from == to {
			// Self-loop after rewrite — silently drop.
			if err := tx.DeleteRelationship(r.ID()); err != nil {
				return err
			}
			continue
		}
		// Rewrite by deleting old and inserting a new edge with
		// the same id and metadata. Re-validates through model.
		newRel, err := model.NewRelationship(r.ID(), from, to, r.Type(),
			r.Certainty(), r.Continuity(), model.RelationshipOptions{
				TimeRange: r.TimeRange(),
				Notes:     r.Notes(),
				Sources:   r.Sources(),
			})
		if err != nil {
			return err
		}
		if err := tx.DeleteRelationship(r.ID()); err != nil {
			return err
		}
		if err := tx.PutRelationship(newRel); err != nil {
			return err
		}
	}
	return tx.DeletePerson(secondary)
}

// applySameAs records a non-destructive same-as link. In this
// reference implementation the link is encoded as a marriage-
// adjacent index would be too misleading; instead we attach a
// note to the Target person preserving the SecondaryID. A future
// version may add a dedicated SameAs entity kind to the store.
func applySameAs(tx store.WriteTx, p model.Proposal) error {
	target, err := tx.GetPerson(p.TargetID())
	if err != nil {
		return err
	}
	note := target.Notes()
	addition := fmt.Sprintf("[same-as:%s]", p.SecondaryID())
	if note != "" {
		note = note + "\n" + addition
	} else {
		note = addition
	}
	updated, err := model.NewPerson(target.ID(), model.PersonOptions{
		Names:  target.Names(),
		Gender: target.Gender(),
		Birth:  target.Birth(),
		Death:  target.Death(),
		Notes:  note,
	})
	if err != nil {
		return err
	}
	return tx.PutPerson(updated)
}

// wouldCreateCycle returns true if a parent->child edge from
// `parent` to `child` would make `parent` a descendant of itself.
// It performs a bounded DFS over child's descendants searching
// for `parent`.
func wouldCreateCycle(tx store.WriteTx, parent, child model.ID) (bool, error) {
	if parent == child {
		return true, nil
	}
	visited := map[model.ID]struct{}{}
	stack := []model.ID{child}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, seen := visited[n]; seen {
			continue
		}
		visited[n] = struct{}{}
		var found bool
		var iterErr error
		err := tx.IterateChildren(n, func(rel model.Relationship) bool {
			if rel.To() == parent {
				found = true
				return false
			}
			stack = append(stack, rel.To())
			return true
		})
		if err != nil {
			iterErr = err
		}
		if iterErr != nil {
			return false, iterErr
		}
		if found {
			return true, nil
		}
	}
	return false, nil
}
