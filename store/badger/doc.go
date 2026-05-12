// SPDX-License-Identifier: AGPL-3.0-or-later

// Package badger provides a persistent on-disk implementation of
// the Linea Store interface backed by Dgraph's BadgerDB embedded
// key-value store.
//
// The adapter is the recommended choice for production deployments
// of Linea-core. It supports the full Store contract with these
// caveats:
//
//   - ViewAt(v) only resolves v == CurrentVersion. Historical
//     point-in-time snapshots across versions are not yet
//     persisted; queries against older versions return
//     errors.CodeVersionNotFound. (The memory adapter retains
//     in-memory history; future work will mirror that here.)
//
//   - Within a single process lifetime, Badger's MVCC guarantees
//     that a ReadTx opened before a write does NOT observe that
//     write — which is exactly what the conformance suite
//     requires.
package badger
