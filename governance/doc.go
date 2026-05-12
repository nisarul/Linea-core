// SPDX-License-Identifier: AGPL-3.0-or-later

// Package governance implements the only legal way to mutate the
// Linea graph: through proposals (CCGGS §8, GGCFS §7).
//
// Application code SHOULD NOT call store.WriteTx mutators
// directly except for:
//   - bootstrapping / ingest tooling that imports a pre-vetted
//     dataset (and is itself recorded as governance metadata), or
//   - administrative recovery operations.
//
// Everything else flows through this package's StateMachine and
// Apply functions, which together guarantee:
//
//   - The proposal state-machine transitions defined in
//     CCGGS §8.3 are the only legal transitions.
//   - Terminal states (Accepted, Rejected, Withdrawn) are
//     immutable.
//   - Reject transitions REQUIRE a non-empty reason.
//   - Accepted proposals advance the graph version atomically
//     with their entity mutations.
//   - Merge and SameAsLink proposals operate on Persons only and
//     preserve audit history (CCGGS §11.3).
package governance
