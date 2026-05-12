// SPDX-License-Identifier: AGPL-3.0-or-later

// Package explain produces structured, presentation-layer-agnostic
// explanations of query results, per CCGGS §9.6 and GGCFS §9.
//
// Explanations are *semantic*, not textual: they expose the
// per-step certainty, continuity (with gap size), source
// citations, the weakest-link edge, the nearest-known-common-
// ancestor, and the graph version that produced the result.
// Localization of any user-facing text is the consumer's job.
package explain
