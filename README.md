# Linea-core

Go reference implementation of the Linea genealogical graph framework.

This library implements the [Linea Specifications](https://github.com/nisarul/Linea-specs):
- Common Core Genealogical Graph Specification (CCGGS)
- Genealogical Graph Core Framework Specification (GGCFS)

The pinned spec version is recorded in [`specs/`](./specs) (git submodule) and exposed as `linea.SpecVersion`.

> Linea — lineage, without assumptions.

## Status

Pre-release. Implements spec v1.1.0.

## Packages

| Package | Purpose |
|---|---|
| `model` | Person, Relationship, Source, Proposal, Certainty, Continuity (typed, invariant-enforced) |
| `errors` | Semantic error codes (e.g. `NO_KNOWN_CONNECTION`) |
| `store` | `Store` port + `memory` and `badger` adapters |
| `versioning` | Monotonic graph version, point-in-time snapshots |
| `governance` | Proposal state machine, merges, same-as-links |
| `provenance` | Source entity model and attachment |
| `graph` | Traversal primitives, cycle prevention |
| `query` | Path enumeration, weakest-link algebra, ranking, NKCA |
| `explain` | Step-by-step explanation generation |

## Build

```sh
git clone --recurse-submodules https://github.com/nisarul/Linea-core.git
cd Linea-core
go test ./...
```

## License

AGPL-3.0-or-later. See [LICENSE](./LICENSE).
The Linea specifications themselves are licensed under CC BY 4.0.
