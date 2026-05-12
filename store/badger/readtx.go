// SPDX-License-Identifier: AGPL-3.0-or-later

package badger

import (
	"bytes"
	"fmt"

	badgerdb "github.com/dgraph-io/badger/v4"

	lerrors "github.com/nisarul/Linea-core/errors"
	"github.com/nisarul/Linea-core/model"
	"github.com/nisarul/Linea-core/store"
)

// readTx is a read-only Badger transaction implementing store.ReadTx.
type readTx struct {
	tx      *badgerdb.Txn
	version store.Version
}

func (r *readTx) Version() store.Version { return r.version }

func (r *readTx) Close() error {
	if r.tx != nil {
		r.tx.Discard()
		r.tx = nil
	}
	return nil
}

// fetch loads the value for k and runs decode on it.
// Returns notFoundCode when k is absent.
func (r *readTx) fetch(k []byte, notFoundCode lerrors.Code, label string,
	decode func([]byte) error) error {
	item, err := r.tx.Get(k)
	if err == badgerdb.ErrKeyNotFound {
		return lerrors.New(notFoundCode, label)
	}
	if err != nil {
		return err
	}
	return item.Value(func(buf []byte) error { return decode(buf) })
}

func (r *readTx) GetPerson(id model.ID) (model.Person, error) {
	var p model.Person
	err := r.fetch(personKey(id.String()), lerrors.CodePersonNotFound,
		fmt.Sprintf("person %s", id), func(buf []byte) error {
			out, e := decodePerson(buf)
			p = out
			return e
		})
	return p, err
}

func (r *readTx) GetRelationship(id model.ID) (model.Relationship, error) {
	var rel model.Relationship
	err := r.fetch(relKey(id.String()), lerrors.CodeRelationshipNotFound,
		fmt.Sprintf("relationship %s", id), func(buf []byte) error {
			out, e := decodeRelationship(buf)
			rel = out
			return e
		})
	return rel, err
}

func (r *readTx) GetSource(id model.ID) (model.Source, error) {
	var s model.Source
	err := r.fetch(sourceKey(id.String()), lerrors.CodeSourceNotFound,
		fmt.Sprintf("source %s", id), func(buf []byte) error {
			out, e := decodeSource(buf)
			s = out
			return e
		})
	return s, err
}

func (r *readTx) GetProposal(id model.ID) (model.Proposal, error) {
	var p model.Proposal
	err := r.fetch(proposalKey(id.String()), lerrors.CodeProposalNotFound,
		fmt.Sprintf("proposal %s", id), func(buf []byte) error {
			out, e := decodeProposal(buf)
			p = out
			return e
		})
	return p, err
}

// scanPrefix iterates all keys under prefix and yields each
// associated relationship by decoding it from the relationship
// store. Used for the parent/child/marriage indices.
func (r *readTx) scanRelIndex(prefix []byte, yield func(model.Relationship) bool) error {
	it := r.tx.NewIterator(badgerdb.IteratorOptions{
		PrefetchValues: false,
		Prefix:         prefix,
	})
	defer it.Close()

	for it.Rewind(); it.Valid(); it.Next() {
		k := it.Item().KeyCopy(nil)
		// Extract the relationship id (last segment after final '/')
		idx := bytes.LastIndexByte(k, sep)
		if idx < 0 || idx == len(k)-1 {
			continue
		}
		rid := string(k[idx+1:])
		rel, err := r.GetRelationship(model.ID(rid))
		if err != nil {
			// Skip stale index entries silently.
			if lerrors.HasCode(err, lerrors.CodeRelationshipNotFound) {
				continue
			}
			return err
		}
		if !yield(rel) {
			return nil
		}
	}
	return nil
}

func (r *readTx) IterateChildren(parent model.ID, yield func(model.Relationship) bool) error {
	return r.scanRelIndex(pcFromPrefix(parent.String()), yield)
}

func (r *readTx) IterateParents(child model.ID, yield func(model.Relationship) bool) error {
	return r.scanRelIndex(pcToPrefix(child.String()), yield)
}

func (r *readTx) IterateMarriages(person model.ID, yield func(model.Relationship) bool) error {
	return r.scanRelIndex(marriagePrefix(person.String()), yield)
}

// scanByPrefix calls decode on every value whose key has the
// supplied byte prefix (the entity prefix + sep).
func scanByPrefix[T any](
	tx *badgerdb.Txn, prefix []byte,
	decode func([]byte) (T, error), yield func(T) bool,
) error {
	it := tx.NewIterator(badgerdb.IteratorOptions{
		PrefetchValues: true,
		Prefix:         prefix,
	})
	defer it.Close()

	for it.Rewind(); it.Valid(); it.Next() {
		var v T
		err := it.Item().Value(func(buf []byte) error {
			out, e := decode(buf)
			v = out
			return e
		})
		if err != nil {
			return err
		}
		if !yield(v) {
			return nil
		}
	}
	return nil
}

func (r *readTx) IteratePersons(yield func(model.Person) bool) error {
	return scanByPrefix(r.tx, []byte{prefixPerson, sep}, decodePerson, yield)
}

func (r *readTx) IterateRelationships(yield func(model.Relationship) bool) error {
	return scanByPrefix(r.tx, []byte{prefixRel, sep}, decodeRelationship, yield)
}

func (r *readTx) IterateSources(yield func(model.Source) bool) error {
	return scanByPrefix(r.tx, []byte{prefixSource, sep}, decodeSource, yield)
}

func (r *readTx) IterateProposals(yield func(model.Proposal) bool) error {
	return scanByPrefix(r.tx, []byte{prefixProposal, sep}, decodeProposal, yield)
}
