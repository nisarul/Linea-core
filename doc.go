// SPDX-License-Identifier: AGPL-3.0-or-later

// Package linea is the Go reference implementation of the Linea
// genealogical graph framework, defined by the Linea Specifications
// (CCGGS + GGCFS) at https://github.com/nisarul/Linea-specs.
//
// Linea models human lineage as a knowledge graph that prefers
// epistemic honesty over completeness: uncertainty, gaps, and
// "no known connection" are first-class outcomes, never hidden.
//
// See sub-packages for the concrete model, storage, governance,
// query, and explanation engines.
package linea

// SpecVersion is the version of the Linea Specifications this
// implementation declares conformance to. It MUST match the tag
// the specs/ git submodule is pinned at.
const SpecVersion = "v1.1.0"
