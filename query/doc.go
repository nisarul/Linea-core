// SPDX-License-Identifier: AGPL-3.0-or-later

// Package query implements the genealogical reasoning engine
// defined in CCGGS §9 and GGCFS §8: path enumeration limited to
// Parent→Child and Marriage edges, weakest-link certainty
// algebra, the 5-criteria lexicographic ranking, nearest known
// common ancestor identification, and the NO_KNOWN_CONNECTION
// outcome.
//
// The engine is purely a reader: it operates against a
// store.ReadTx snapshot. All results are stamped with the graph
// version they were evaluated against (CCGGS §8.5).
package query
