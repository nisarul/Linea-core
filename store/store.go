// SPDX-License-Identifier: AGPL-3.0-or-later

// Package store defines the storage port (interface) for the
// Linea engine and provides reference adapter implementations.
//
// The Store port is intentionally narrow: it exposes only the
// primitives needed by higher layers (graph traversal, query,
// governance). All higher-level semantics — proposal lifecycle,
// path ranking, certainty algebra — live elsewhere and depend
// only on this interface, never on any concrete adapter.
//
// Two reference adapters ship with the core:
//
//   - store/memory  — in-memory implementation; the canonical
//     adapter for unit tests.
//   - store/badger  — embedded persistent KV implementation;
//     the recommended production adapter.
//
// New adapters MUST satisfy the same Store interface and pass
// the shared conformance tests in store/storetest.
package store

import (
	"context"

	"github.com/nisarul/Linea-core/model"
)

// Version identifies a monotonic snapshot of the graph.
//
// Every accepted proposal advances the version by one; queries
// SHOULD record the version they evaluated against (CCGGS §8.5,
// GGCFS §7.3). Version zero is the empty initial graph.
type Version uint64

// Store is the persistence port for the Linea graph.
//
// All read methods MUST be safe for concurrent use by multiple
// goroutines. Write methods MUST be atomic per call: a failed
// write must leave the store in its prior consistent state.
//
// Adapters MUST NOT silently mutate or filter data. Every
// retrieval MUST return exactly what was stored, byte-identical.
type Store interface {
	// CurrentVersion returns the latest committed graph version.
	CurrentVersion(ctx context.Context) (Version, error)

	// View opens a read-only snapshot at the latest committed
	// version. The returned ReadTx MUST be closed by the caller.
	View(ctx context.Context) (ReadTx, error)

	// ViewAt opens a read-only snapshot at a specific historical
	// version. Returns errors.CodeVersionNotFound if the version
	// does not exist.
	ViewAt(ctx context.Context, v Version) (ReadTx, error)

	// Update opens a read-write transaction. The function f is
	// invoked with a WriteTx; if it returns nil the transaction
	// is committed, advancing CurrentVersion by one. If f returns
	// a non-nil error the transaction is rolled back and the
	// error is propagated.
	//
	// Implementations MUST ensure that concurrent Update calls
	// are serialised — only one writer at a time may commit.
	Update(ctx context.Context, f func(tx WriteTx) error) (Version, error)

	// Close releases all resources held by the adapter.
	// It is safe to call Close more than once.
	Close() error
}

// ReadTx is a read-only view of the graph at a fixed version.
//
// Iterators returned by ReadTx remain valid for the life of the
// transaction. Callers MUST call Close to release resources.
type ReadTx interface {
	// Version returns the graph version this transaction reads.
	Version() Version

	// GetPerson returns the Person with the given ID.
	// Returns *errors.Error with CodePersonNotFound if absent.
	GetPerson(id model.ID) (model.Person, error)

	// GetRelationship returns the Relationship with the given ID.
	// Returns *errors.Error with CodeRelationshipNotFound if absent.
	GetRelationship(id model.ID) (model.Relationship, error)

	// GetSource returns the Source with the given ID.
	// Returns *errors.Error with CodeSourceNotFound if absent.
	GetSource(id model.ID) (model.Source, error)

	// GetProposal returns the Proposal with the given ID.
	// Returns *errors.Error with CodeProposalNotFound if absent.
	GetProposal(id model.ID) (model.Proposal, error)

	// IterateChildren yields all relationships of type
	// RelTypeParentChild whose From() == parent. The order is
	// adapter-defined but stable within a single transaction.
	IterateChildren(parent model.ID, yield func(model.Relationship) bool) error

	// IterateParents yields all relationships of type
	// RelTypeParentChild whose To() == child.
	IterateParents(child model.ID, yield func(model.Relationship) bool) error

	// IterateMarriages yields all RelTypeMarriage relationships
	// touching the given person (as From or To).
	IterateMarriages(person model.ID, yield func(model.Relationship) bool) error

	// IteratePersons yields every Person in the graph.
	IteratePersons(yield func(model.Person) bool) error

	// IterateRelationships yields every Relationship in the graph.
	IterateRelationships(yield func(model.Relationship) bool) error

	// IterateProposals yields every Proposal regardless of state.
	IterateProposals(yield func(model.Proposal) bool) error

	// Close releases the transaction's resources.
	Close() error
}

// WriteTx extends ReadTx with mutation primitives.
//
// All mutations are visible to subsequent reads on the same
// transaction. They become durable only when the surrounding
// Update call's function returns nil.
type WriteTx interface {
	ReadTx

	// PutPerson inserts or replaces a Person.
	PutPerson(p model.Person) error

	// DeletePerson removes a Person. It does not cascade: the
	// caller MUST remove any incident relationships first.
	// Returns CodePersonNotFound if absent.
	DeletePerson(id model.ID) error

	// PutRelationship inserts or replaces a Relationship.
	// The adapter MUST verify that both endpoints exist; otherwise
	// it returns CodePersonNotFound for the missing endpoint.
	PutRelationship(r model.Relationship) error

	// DeleteRelationship removes a Relationship.
	// Returns CodeRelationshipNotFound if absent.
	DeleteRelationship(id model.ID) error

	// PutSource inserts or replaces a Source.
	PutSource(s model.Source) error

	// DeleteSource removes a Source.
	// Returns CodeSourceNotFound if absent.
	DeleteSource(id model.ID) error

	// PutProposal inserts or replaces a Proposal.
	// Adapters MUST refuse writes that mutate a Proposal already
	// in a terminal state (CCGGS §8.3); enforcement happens in
	// package governance, but adapters SHOULD double-check.
	PutProposal(p model.Proposal) error
}
