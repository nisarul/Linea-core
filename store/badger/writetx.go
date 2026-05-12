// SPDX-License-Identifier: AGPL-3.0-or-later

package badger

import (
	"fmt"

	badgerdb "github.com/dgraph-io/badger/v4"

	lerrors "github.com/nisarul/Linea-core/errors"
	"github.com/nisarul/Linea-core/model"
)

// writeTx is a read-write Badger transaction implementing store.WriteTx.
type writeTx struct {
	readTx
}

// PutPerson inserts or replaces a Person.
func (w *writeTx) PutPerson(p model.Person) error {
	if p.ID().IsZero() {
		return lerrors.New(lerrors.CodeInvalidArgument, "person id required")
	}
	buf, err := encodePerson(p)
	if err != nil {
		return err
	}
	return w.tx.Set(personKey(p.ID().String()), buf)
}

// DeletePerson removes a Person. The caller is responsible for
// removing dependent relationships first (per Store contract).
func (w *writeTx) DeletePerson(id model.ID) error {
	k := personKey(id.String())
	if _, err := w.tx.Get(k); err != nil {
		if err == badgerdb.ErrKeyNotFound {
			return lerrors.New(lerrors.CodePersonNotFound, fmt.Sprintf("person %s", id))
		}
		return err
	}
	return w.tx.Delete(k)
}

// PutRelationship inserts or replaces a Relationship and
// maintains its incident indices.
func (w *writeTx) PutRelationship(r model.Relationship) error {
	if r.ID().IsZero() {
		return lerrors.New(lerrors.CodeInvalidArgument, "relationship id required")
	}
	if err := w.requirePersonExists(r.From(), r.ID()); err != nil {
		return err
	}
	if err := w.requirePersonExists(r.To(), r.ID()); err != nil {
		return err
	}

	// If a prior version exists with different endpoints we must
	// remove its index entries before installing new ones.
	if prev, err := w.GetRelationship(r.ID()); err == nil {
		if err := w.removeRelIndices(prev); err != nil {
			return err
		}
	}

	buf, err := encodeRelationship(r)
	if err != nil {
		return err
	}
	if err := w.tx.Set(relKey(r.ID().String()), buf); err != nil {
		return err
	}
	return w.writeRelIndices(r)
}

// DeleteRelationship removes a Relationship and its indices.
func (w *writeTx) DeleteRelationship(id model.ID) error {
	k := relKey(id.String())
	prev, err := w.GetRelationship(id)
	if err != nil {
		return err
	}
	if err := w.removeRelIndices(prev); err != nil {
		return err
	}
	return w.tx.Delete(k)
}

// PutSource inserts or replaces a Source.
func (w *writeTx) PutSource(s model.Source) error {
	if s.ID().IsZero() {
		return lerrors.New(lerrors.CodeInvalidArgument, "source id required")
	}
	buf, err := encodeSource(s)
	if err != nil {
		return err
	}
	return w.tx.Set(sourceKey(s.ID().String()), buf)
}

// DeleteSource removes a Source.
func (w *writeTx) DeleteSource(id model.ID) error {
	k := sourceKey(id.String())
	if _, err := w.tx.Get(k); err != nil {
		if err == badgerdb.ErrKeyNotFound {
			return lerrors.New(lerrors.CodeSourceNotFound, fmt.Sprintf("source %s", id))
		}
		return err
	}
	return w.tx.Delete(k)
}

// PutProposal inserts or replaces a Proposal, refusing to mutate
// a proposal that is already in a terminal state into a different
// state (CCGGS §8.3).
func (w *writeTx) PutProposal(p model.Proposal) error {
	if p.ID().IsZero() {
		return lerrors.New(lerrors.CodeInvalidArgument, "proposal id required")
	}
	if existing, err := w.GetProposal(p.ID()); err == nil {
		if existing.State().IsTerminal() && existing.State() != p.State() {
			return lerrors.New(lerrors.CodeImmutableTerminalProposal,
				fmt.Sprintf("proposal %s is in terminal state %s", p.ID(), existing.State()))
		}
	}
	buf, err := encodeProposal(p)
	if err != nil {
		return err
	}
	return w.tx.Set(proposalKey(p.ID().String()), buf)
}

// requirePersonExists returns CodePersonNotFound (scoped to the
// supplied relationship id) when person is not in the store.
func (w *writeTx) requirePersonExists(person, relID model.ID) error {
	if _, err := w.tx.Get(personKey(person.String())); err != nil {
		if err == badgerdb.ErrKeyNotFound {
			return lerrors.New(lerrors.CodePersonNotFound,
				fmt.Sprintf("relationship %s: person %s missing", relID, person))
		}
		return err
	}
	return nil
}

func (w *writeTx) writeRelIndices(r model.Relationship) error {
	rid := r.ID().String()
	switch r.Type() {
	case model.RelTypeParentChild:
		if err := w.tx.Set(pcFromIndexKey(r.From().String(), rid), nil); err != nil {
			return err
		}
		return w.tx.Set(pcToIndexKey(r.To().String(), rid), nil)
	case model.RelTypeMarriage:
		if err := w.tx.Set(marriageIndexKey(r.From().String(), rid), nil); err != nil {
			return err
		}
		return w.tx.Set(marriageIndexKey(r.To().String(), rid), nil)
	}
	return nil
}

func (w *writeTx) removeRelIndices(r model.Relationship) error {
	rid := r.ID().String()
	switch r.Type() {
	case model.RelTypeParentChild:
		if err := w.tx.Delete(pcFromIndexKey(r.From().String(), rid)); err != nil {
			return err
		}
		return w.tx.Delete(pcToIndexKey(r.To().String(), rid))
	case model.RelTypeMarriage:
		if err := w.tx.Delete(marriageIndexKey(r.From().String(), rid)); err != nil {
			return err
		}
		return w.tx.Delete(marriageIndexKey(r.To().String(), rid))
	}
	return nil
}
