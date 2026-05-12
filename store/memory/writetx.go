// SPDX-License-Identifier: AGPL-3.0-or-later

package memory

import (
	"fmt"

	lerrors "github.com/nisarul/Linea-core/errors"
	"github.com/nisarul/Linea-core/model"
	"github.com/nisarul/Linea-core/store"
)

// writeTx applies copy-on-write mutations to a base state. The
// changes become durable only when commitState is called.
type writeTx struct {
	readTx
	base *state // never modified
	// Per-entity overlay maps. A nil value means "deleted".
	personOver map[model.ID]*model.Person
	relOver    map[model.ID]*model.Relationship
	srcOver    map[model.ID]*model.Source
	propOver   map[model.ID]*model.Proposal
}

func newWriteTx(base *state) *writeTx {
	tx := &writeTx{
		base:       base,
		personOver: make(map[model.ID]*model.Person),
		relOver:    make(map[model.ID]*model.Relationship),
		srcOver:    make(map[model.ID]*model.Source),
		propOver:   make(map[model.ID]*model.Proposal),
	}
	tx.state = base
	return tx
}

// Override readTx getters so reads inside the transaction see
// pending writes immediately.

func (w *writeTx) GetPerson(id model.ID) (model.Person, error) {
	if v, ok := w.personOver[id]; ok {
		if v == nil {
			return model.Person{}, lerrors.New(lerrors.CodePersonNotFound,
				fmt.Sprintf("person %s", id))
		}
		return *v, nil
	}
	return w.readTx.GetPerson(id)
}

func (w *writeTx) GetRelationship(id model.ID) (model.Relationship, error) {
	if v, ok := w.relOver[id]; ok {
		if v == nil {
			return model.Relationship{}, lerrors.New(lerrors.CodeRelationshipNotFound,
				fmt.Sprintf("relationship %s", id))
		}
		return *v, nil
	}
	return w.readTx.GetRelationship(id)
}

func (w *writeTx) GetSource(id model.ID) (model.Source, error) {
	if v, ok := w.srcOver[id]; ok {
		if v == nil {
			return model.Source{}, lerrors.New(lerrors.CodeSourceNotFound,
				fmt.Sprintf("source %s", id))
		}
		return *v, nil
	}
	return w.readTx.GetSource(id)
}

func (w *writeTx) GetProposal(id model.ID) (model.Proposal, error) {
	if v, ok := w.propOver[id]; ok {
		if v == nil {
			return model.Proposal{}, lerrors.New(lerrors.CodeProposalNotFound,
				fmt.Sprintf("proposal %s", id))
		}
		return *v, nil
	}
	return w.readTx.GetProposal(id)
}

// hasPerson is used to validate relationship endpoints during PutRelationship.
func (w *writeTx) hasPerson(id model.ID) bool {
	if v, ok := w.personOver[id]; ok {
		return v != nil
	}
	_, ok := w.base.persons[id]
	return ok
}

// PutPerson inserts or replaces a Person.
func (w *writeTx) PutPerson(p model.Person) error {
	if p.ID().IsZero() {
		return lerrors.New(lerrors.CodeInvalidArgument, "person id required")
	}
	cp := p
	w.personOver[p.ID()] = &cp
	return nil
}

// DeletePerson removes a Person.
func (w *writeTx) DeletePerson(id model.ID) error {
	if _, err := w.GetPerson(id); err != nil {
		return err
	}
	w.personOver[id] = nil
	return nil
}

// PutRelationship inserts or replaces a Relationship after
// validating that both endpoints exist in the current view.
func (w *writeTx) PutRelationship(r model.Relationship) error {
	if r.ID().IsZero() {
		return lerrors.New(lerrors.CodeInvalidArgument, "relationship id required")
	}
	if !w.hasPerson(r.From()) {
		return lerrors.New(lerrors.CodePersonNotFound,
			fmt.Sprintf("relationship %s: from-person %s missing", r.ID(), r.From()))
	}
	if !w.hasPerson(r.To()) {
		return lerrors.New(lerrors.CodePersonNotFound,
			fmt.Sprintf("relationship %s: to-person %s missing", r.ID(), r.To()))
	}
	cp := r
	w.relOver[r.ID()] = &cp
	return nil
}

// DeleteRelationship removes a Relationship.
func (w *writeTx) DeleteRelationship(id model.ID) error {
	if _, err := w.GetRelationship(id); err != nil {
		return err
	}
	w.relOver[id] = nil
	return nil
}

// PutSource inserts or replaces a Source.
func (w *writeTx) PutSource(s model.Source) error {
	if s.ID().IsZero() {
		return lerrors.New(lerrors.CodeInvalidArgument, "source id required")
	}
	cp := s
	w.srcOver[s.ID()] = &cp
	return nil
}

// DeleteSource removes a Source.
func (w *writeTx) DeleteSource(id model.ID) error {
	if _, err := w.GetSource(id); err != nil {
		return err
	}
	w.srcOver[id] = nil
	return nil
}

// PutProposal inserts or replaces a Proposal. Adapters double-
// check the terminal-state rule per CCGGS §8.3.
func (w *writeTx) PutProposal(p model.Proposal) error {
	if p.ID().IsZero() {
		return lerrors.New(lerrors.CodeInvalidArgument, "proposal id required")
	}
	if existing, ok := w.base.proposals[p.ID()]; ok {
		if existing.State().IsTerminal() && existing.State() != p.State() {
			return lerrors.New(lerrors.CodeImmutableTerminalProposal,
				fmt.Sprintf("proposal %s is in terminal state %s", p.ID(), existing.State()))
		}
	}
	cp := p
	w.propOver[p.ID()] = &cp
	return nil
}

// commitState merges the overlays into a new immutable state at
// the supplied version.
func (w *writeTx) commitState(version store.Version) *state {
	out := &state{
		version:       version,
		persons:       cloneMap(w.base.persons),
		relationships: cloneMap(w.base.relationships),
		sources:       cloneMap(w.base.sources),
		proposals:     cloneMap(w.base.proposals),
	}
	applyOverlay(out.persons, w.personOver)
	applyOverlay(out.relationships, w.relOver)
	applyOverlay(out.sources, w.srcOver)
	applyOverlay(out.proposals, w.propOver)
	return out
}

func cloneMap[V any](in map[model.ID]V) map[model.ID]V {
	out := make(map[model.ID]V, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func applyOverlay[V any](dst map[model.ID]V, over map[model.ID]*V) {
	for k, v := range over {
		if v == nil {
			delete(dst, k)
			continue
		}
		dst[k] = *v
	}
}
