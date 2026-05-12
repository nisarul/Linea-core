// SPDX-License-Identifier: AGPL-3.0-or-later

package query

import (
	"context"
	"fmt"
	"sort"

	lerrors "github.com/nisarul/Linea-core/errors"
	"github.com/nisarul/Linea-core/model"
	"github.com/nisarul/Linea-core/store"
)

// DefaultMaxDepth caps the depth of path enumeration to keep
// queries bounded. It can be overridden via Options.MaxDepth.
const DefaultMaxDepth = 16

// Options control path enumeration and ranking.
type Options struct {
	// MaxDepth limits path length (number of edges). Zero means
	// DefaultMaxDepth.
	MaxDepth int
	// MaxPaths limits how many paths Find returns after ranking.
	// Zero means "all paths up to MaxDepth".
	MaxPaths int
	// IncludeAffinal allows Marriage edges in the search. When
	// false, only Parent↔Child traversal is permitted (queries
	// for biological lineage only).
	IncludeAffinal bool
}

func (o Options) maxDepth() int {
	if o.MaxDepth <= 0 {
		return DefaultMaxDepth
	}
	return o.MaxDepth
}

// FindPaths enumerates all valid paths between `from` and `to`
// in the supplied snapshot, ranks them per CCGGS §9.3, and
// returns them in best-first order.
//
// Returns errors.ErrNoKnownConnection if no path exists; this is
// a first-class semantic outcome (CCGGS §9.2), not a failure.
func FindPaths(
	_ context.Context,
	rtx store.ReadTx,
	from, to model.ID,
	opts Options,
) ([]Path, error) {
	if from.IsZero() || to.IsZero() {
		return nil, lerrors.New(lerrors.CodeInvalidArgument, "from/to required")
	}
	if from == to {
		return nil, lerrors.New(lerrors.CodeInvalidArgument, "from and to must differ")
	}
	// Validate both endpoints exist.
	if _, err := rtx.GetPerson(from); err != nil {
		return nil, err
	}
	if _, err := rtx.GetPerson(to); err != nil {
		return nil, err
	}

	maxDepth := opts.maxDepth()
	var found []Path
	visited := map[model.ID]bool{from: true}
	steps := make([]Step, 0, maxDepth)

	var dfs func(node model.ID)
	dfs = func(node model.ID) {
		if len(steps) >= maxDepth {
			return
		}
		// Expand parent->child outgoing edges (forward).
		_ = rtx.IterateChildren(node, func(rel model.Relationship) bool {
			next := rel.To()
			if visited[next] {
				return true
			}
			steps = append(steps, Step{
				Relationship: rel, Direction: EdgeForward,
				FromPerson: node, ToPerson: next,
			})
			visited[next] = true
			if next == to {
				found = append(found, makePath(rtx, steps))
			} else {
				dfs(next)
			}
			delete(visited, next)
			steps = steps[:len(steps)-1]
			return true
		})
		// Expand parent->child incoming edges (reverse: walk to a parent).
		_ = rtx.IterateParents(node, func(rel model.Relationship) bool {
			next := rel.From()
			if visited[next] {
				return true
			}
			steps = append(steps, Step{
				Relationship: rel, Direction: EdgeReverse,
				FromPerson: node, ToPerson: next,
			})
			visited[next] = true
			if next == to {
				found = append(found, makePath(rtx, steps))
			} else {
				dfs(next)
			}
			delete(visited, next)
			steps = steps[:len(steps)-1]
			return true
		})
		// Expand marriage edges.
		if opts.IncludeAffinal {
			_ = rtx.IterateMarriages(node, func(rel model.Relationship) bool {
				next := rel.To()
				dir := EdgeForward
				if next == node {
					next = rel.From()
					dir = EdgeReverse
				}
				if visited[next] {
					return true
				}
				steps = append(steps, Step{
					Relationship: rel, Direction: dir,
					FromPerson: node, ToPerson: next,
				})
				visited[next] = true
				if next == to {
					found = append(found, makePath(rtx, steps))
				} else {
					dfs(next)
				}
				delete(visited, next)
				steps = steps[:len(steps)-1]
				return true
			})
		}
	}
	dfs(from)

	if len(found) == 0 {
		return nil, lerrors.ErrNoKnownConnection
	}

	// Per CCGGS §9.5: lineage explanations are preferred. We
	// surface them first by sorting with classification as the
	// outermost criterion, then the standard ranking.
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].Classification != found[j].Classification {
			return found[i].Classification == PathLineage
		}
		return Less(found[i], found[j])
	})

	if opts.MaxPaths > 0 && len(found) > opts.MaxPaths {
		found = found[:opts.MaxPaths]
	}
	return found, nil
}

func makePath(rtx store.ReadTx, steps []Step) Path {
	cp := make([]Step, len(steps))
	copy(cp, steps)
	cert, total, gaps, length, class := computeAggregate(cp)
	return Path{
		Steps:          cp,
		Certainty:      cert,
		TotalGap:       total,
		GapEdges:       gaps,
		Length:         length,
		Classification: class,
		GraphVersion:   rtx.Version(),
	}
}

// CommonAncestor describes the result of NearestKnownCommonAncestor.
type CommonAncestor struct {
	// AncestorID is the nearest known common ancestor (NKCA).
	AncestorID model.ID
	// Unknown is true if the NKCA is an unknown-ancestor placeholder.
	Unknown bool
	// PathFromA is the lineage path from `a` up to the NKCA.
	PathFromA Path
	// PathFromB is the lineage path from `b` up to the NKCA.
	PathFromB Path
	// CombinedCertainty is the weakest-link across both paths.
	CombinedCertainty model.Certainty
	// TotalGenerations is the sum of generation distances on both paths.
	TotalGenerations int
	// GraphVersion stamps the snapshot the result was computed against.
	GraphVersion store.Version
}

// NearestKnownCommonAncestor implements CCGGS §9.6.
//
// It walks ancestors of both persons (Parent→Child reversed) and
// returns the common ancestor minimising the sum of generations
// (counting gap-generations on Gapped edges). Ties broken by
// higher combined certainty, then by stable id ordering.
//
// Returns ErrNoKnownConnection if no common ancestor exists in
// the snapshot.
func NearestKnownCommonAncestor(
	ctx context.Context,
	rtx store.ReadTx,
	a, b model.ID,
	opts Options,
) (*CommonAncestor, error) {
	if a == b {
		return nil, lerrors.New(lerrors.CodeInvalidArgument, "a and b must differ")
	}
	ancestorsA, err := ancestorPaths(rtx, a, opts)
	if err != nil {
		return nil, err
	}
	ancestorsB, err := ancestorPaths(rtx, b, opts)
	if err != nil {
		return nil, err
	}
	// Self-ancestor inclusion: a person counts as an ancestor of
	// themselves at distance 0 with Certain certainty.
	ancestorsA[a] = ancestorEntry{path: Path{GraphVersion: rtx.Version()}, dist: 0, cert: model.CertaintyCertain}
	ancestorsB[b] = ancestorEntry{path: Path{GraphVersion: rtx.Version()}, dist: 0, cert: model.CertaintyCertain}

	var best *CommonAncestor
	for id, ea := range ancestorsA {
		eb, ok := ancestorsB[id]
		if !ok {
			continue
		}
		// Note: id == a with eb.dist == 0 means a is an ancestor of b
		// (and symmetrically). That outcome is desired — we keep the
		// candidate so the caller learns about the direct lineage.
		total := ea.dist + eb.dist
		combined := ea.cert.Min(eb.cert)
		anc, err := rtx.GetPerson(id)
		if err != nil {
			return nil, err
		}
		candidate := &CommonAncestor{
			AncestorID:        id,
			Unknown:           anc.IsUnknownAncestor(),
			PathFromA:         ea.path,
			PathFromB:         eb.path,
			CombinedCertainty: combined,
			TotalGenerations:  total,
			GraphVersion:      rtx.Version(),
		}
		if best == nil ||
			candidate.TotalGenerations < best.TotalGenerations ||
			(candidate.TotalGenerations == best.TotalGenerations &&
				candidate.CombinedCertainty.Rank() > best.CombinedCertainty.Rank()) ||
			(candidate.TotalGenerations == best.TotalGenerations &&
				candidate.CombinedCertainty == best.CombinedCertainty &&
				candidate.AncestorID < best.AncestorID) {
			best = candidate
		}
	}
	if best == nil {
		return nil, lerrors.ErrNoKnownConnection
	}
	return best, nil
}

type ancestorEntry struct {
	path Path
	dist int
	cert model.Certainty
}

// ancestorPaths returns a map from ancestor-id to the cheapest
// upward path (and its distance + cert) reachable from `from`.
// "Distance" counts generations: each Continuous PC edge = 1,
// Gapped = 1 + gap, Unknown gap = sentinel.
func ancestorPaths(rtx store.ReadTx, from model.ID, opts Options) (map[model.ID]ancestorEntry, error) {
	maxDepth := opts.maxDepth()
	out := map[model.ID]ancestorEntry{}
	if _, err := rtx.GetPerson(from); err != nil {
		return nil, err
	}
	type frame struct {
		id    model.ID
		steps []Step
		dist  int
		cert  model.Certainty
	}
	stack := []frame{{id: from, cert: model.CertaintyCertain}}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if len(f.steps) > maxDepth {
			continue
		}
		// Record/replace if cheaper.
		if f.id != from {
			if cur, ok := out[f.id]; !ok ||
				f.dist < cur.dist ||
				(f.dist == cur.dist && f.cert.Rank() > cur.cert.Rank()) {
				cp := make([]Step, len(f.steps))
				copy(cp, f.steps)
				c, tg, ge, l, cl := computeAggregate(cp)
				out[f.id] = ancestorEntry{
					path: Path{
						Steps:          cp,
						Certainty:      c,
						TotalGap:       tg,
						GapEdges:       ge,
						Length:         l,
						Classification: cl,
						GraphVersion:   rtx.Version(),
					},
					dist: f.dist,
					cert: f.cert,
				}
			}
		}
		_ = rtx.IterateParents(f.id, func(rel model.Relationship) bool {
			parent := rel.From()
			step := Step{
				Relationship: rel, Direction: EdgeReverse,
				FromPerson: f.id, ToPerson: parent,
			}
			weight := 1
			if rel.Continuity().IsGapped() {
				if rel.Continuity().Gap.KnownSize {
					weight = 1 + rel.Continuity().Gap.Size
				} else {
					weight = rel.GapWeight() // sentinel
				}
			}
			next := frame{
				id:    parent,
				steps: append(append([]Step(nil), f.steps...), step),
				dist:  f.dist + weight,
				cert:  f.cert.Min(rel.Certainty()),
			}
			stack = append(stack, next)
			return true
		})
	}
	return out, nil
}

// helper for diagnostics
func (p Path) String() string {
	if len(p.Steps) == 0 {
		return "Path<empty>"
	}
	return fmt.Sprintf("Path<%s..%s, len=%d, cert=%s, gap=%d/%d, %s, v=%d>",
		p.From(), p.To(), p.Length, p.Certainty, p.TotalGap, p.GapEdges, p.Classification, p.GraphVersion)
}
