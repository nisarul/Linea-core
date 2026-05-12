// SPDX-License-Identifier: AGPL-3.0-or-later

// Package model defines the core entity types of the Linea
// genealogical graph. These types enforce the invariants of the
// Linea Specifications (CCGGS + GGCFS) at construction time so
// that no invalid value can flow further into the engine.
//
// All exported types are immutable from the caller's perspective:
// they are constructed via New* constructors that validate their
// inputs and return either a value or a typed error.
package model
