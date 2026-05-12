// SPDX-License-Identifier: AGPL-3.0-or-later

package memory

import (
	"fmt"

	lerrors "github.com/nisarul/Linea-core/errors"
	"github.com/nisarul/Linea-core/model"
	"github.com/nisarul/Linea-core/store"
)

// readTx is a read-only view bound to a specific *state snapshot.
type readTx struct {
	state  *state
	closed bool
}

func (r *readTx) Version() store.Version { return r.state.version }

func (r *readTx) Close() error {
	r.closed = true
	return nil
}

func (r *readTx) GetPerson(id model.ID) (model.Person, error) {
	if p, ok := r.state.persons[id]; ok {
		return p, nil
	}
	return model.Person{}, lerrors.New(lerrors.CodePersonNotFound,
		fmt.Sprintf("person %s", id))
}

func (r *readTx) GetRelationship(id model.ID) (model.Relationship, error) {
	if rel, ok := r.state.relationships[id]; ok {
		return rel, nil
	}
	return model.Relationship{}, lerrors.New(lerrors.CodeRelationshipNotFound,
		fmt.Sprintf("relationship %s", id))
}

func (r *readTx) GetSource(id model.ID) (model.Source, error) {
	if s, ok := r.state.sources[id]; ok {
		return s, nil
	}
	return model.Source{}, lerrors.New(lerrors.CodeSourceNotFound,
		fmt.Sprintf("source %s", id))
}

func (r *readTx) GetProposal(id model.ID) (model.Proposal, error) {
	if p, ok := r.state.proposals[id]; ok {
		return p, nil
	}
	return model.Proposal{}, lerrors.New(lerrors.CodeProposalNotFound,
		fmt.Sprintf("proposal %s", id))
}

func (r *readTx) IterateChildren(parent model.ID, yield func(model.Relationship) bool) error {
	for _, rel := range r.state.relationships {
		if rel.Type() == model.RelTypeParentChild && rel.From() == parent {
			if !yield(rel) {
				return nil
			}
		}
	}
	return nil
}

func (r *readTx) IterateParents(child model.ID, yield func(model.Relationship) bool) error {
	for _, rel := range r.state.relationships {
		if rel.Type() == model.RelTypeParentChild && rel.To() == child {
			if !yield(rel) {
				return nil
			}
		}
	}
	return nil
}

func (r *readTx) IterateMarriages(person model.ID, yield func(model.Relationship) bool) error {
	for _, rel := range r.state.relationships {
		if rel.Type() != model.RelTypeMarriage {
			continue
		}
		if rel.From() == person || rel.To() == person {
			if !yield(rel) {
				return nil
			}
		}
	}
	return nil
}

func (r *readTx) IteratePersons(yield func(model.Person) bool) error {
	for _, p := range r.state.persons {
		if !yield(p) {
			return nil
		}
	}
	return nil
}

func (r *readTx) IterateRelationships(yield func(model.Relationship) bool) error {
	for _, rel := range r.state.relationships {
		if !yield(rel) {
			return nil
		}
	}
	return nil
}

func (r *readTx) IterateSources(yield func(model.Source) bool) error {
	for _, s := range r.state.sources {
		if !yield(s) {
			return nil
		}
	}
	return nil
}

func (r *readTx) IterateProposals(yield func(model.Proposal) bool) error {
	for _, p := range r.state.proposals {
		if !yield(p) {
			return nil
		}
	}
	return nil
}
