// SPDX-License-Identifier: AGPL-3.0-or-later

package badger

// Wire types used to (de)serialise model entities to JSON.
//
// These mirror the public API of the model package without
// exposing any unexported field of the model types directly.
// All decoding paths go through the model's New* constructors,
// so loaded values respect every spec invariant.

import (
	"encoding/json"
	"fmt"

	"github.com/nisarul/Linea-core/model"
)

// ---------- Person ----------

type personWire struct {
	ID      string         `json:"id"`
	Names   []nameWire     `json:"names,omitempty"`
	Gender  string         `json:"gender,omitempty"`
	Birth   *timeRangeWire `json:"birth,omitempty"`
	Death   *timeRangeWire `json:"death,omitempty"`
	Notes   string         `json:"notes,omitempty"`
	Unknown bool           `json:"unknown,omitempty"`
}

type nameWire struct {
	Text      string `json:"text"`
	Language  string `json:"lang,omitempty"`
	Script    string `json:"script,omitempty"`
	Type      string `json:"type,omitempty"`
	Preferred bool   `json:"preferred,omitempty"`
}

type timeRangeWire struct {
	EarliestKnown bool   `json:"e_known,omitempty"`
	Earliest      int    `json:"e,omitempty"`
	LatestKnown   bool   `json:"l_known,omitempty"`
	Latest        int    `json:"l,omitempty"`
	Calendar      string `json:"cal,omitempty"`
	Circa         bool   `json:"circa,omitempty"`
}

func encodePerson(p model.Person) ([]byte, error) {
	w := personWire{
		ID:      p.ID().String(),
		Gender:  string(p.Gender()),
		Notes:   p.Notes(),
		Unknown: p.IsUnknownAncestor(),
	}
	for _, n := range p.Names() {
		w.Names = append(w.Names, nameWire{
			Text:      n.Text,
			Language:  n.Language,
			Script:    n.Script,
			Type:      string(n.Type),
			Preferred: n.Preferred,
		})
	}
	if !p.Birth().IsZero() {
		w.Birth = encodeTimeRange(p.Birth())
	}
	if !p.Death().IsZero() {
		w.Death = encodeTimeRange(p.Death())
	}
	return json.Marshal(w)
}

func decodePerson(buf []byte) (model.Person, error) {
	var w personWire
	if err := json.Unmarshal(buf, &w); err != nil {
		return model.Person{}, fmt.Errorf("decode person: %w", err)
	}
	id, err := model.ParseID(w.ID)
	if err != nil {
		return model.Person{}, fmt.Errorf("decode person: %w", err)
	}
	if w.Unknown {
		return model.NewUnknownAncestor(id)
	}
	names := make([]model.Name, 0, len(w.Names))
	for _, nw := range w.Names {
		n, err := model.NewName(nw.Text, nw.Language, nw.Script, model.NameType(nw.Type), nw.Preferred)
		if err != nil {
			return model.Person{}, fmt.Errorf("decode person %s: %w", id, err)
		}
		names = append(names, n)
	}
	opts := model.PersonOptions{
		Names:  names,
		Gender: model.Gender(w.Gender),
		Notes:  w.Notes,
	}
	if w.Birth != nil {
		opts.Birth = decodeTimeRange(w.Birth)
	}
	if w.Death != nil {
		opts.Death = decodeTimeRange(w.Death)
	}
	return model.NewPerson(id, opts)
}

func encodeTimeRange(tr model.TimeRange) *timeRangeWire {
	return &timeRangeWire{
		EarliestKnown: tr.Earliest.KnownYear,
		Earliest:      tr.Earliest.Year,
		LatestKnown:   tr.Latest.KnownYear,
		Latest:        tr.Latest.Year,
		Calendar:      string(tr.Calendar),
		Circa:         tr.Circa,
	}
}

func decodeTimeRange(w *timeRangeWire) model.TimeRange {
	earliest := model.UnknownYear()
	if w.EarliestKnown {
		earliest = model.KnownYearBound(w.Earliest)
	}
	latest := model.UnknownYear()
	if w.LatestKnown {
		latest = model.KnownYearBound(w.Latest)
	}
	tr, _ := model.NewTimeRange(earliest, latest, model.Calendar(w.Calendar), w.Circa)
	return tr
}

// ---------- Relationship ----------

type relationshipWire struct {
	ID         string         `json:"id"`
	From       string         `json:"from"`
	To         string         `json:"to"`
	Type       int            `json:"type"`
	Certainty  int            `json:"cert"`
	Continuity continuityWire `json:"cont"`
	TimeRange  *timeRangeWire `json:"time,omitempty"`
	Notes      string         `json:"notes,omitempty"`
	Sources    []string       `json:"sources,omitempty"`
}

type continuityWire struct {
	State        int  `json:"state"`
	GapKnown     bool `json:"gk,omitempty"`
	GapSize      int  `json:"gs,omitempty"`
	GapSpecified bool `json:"-"` // not stored; derived from State
}

func encodeRelationship(r model.Relationship) ([]byte, error) {
	cont := continuityWire{State: int(r.Continuity().State)}
	if r.Continuity().IsGapped() {
		cont.GapKnown = r.Continuity().Gap.KnownSize
		cont.GapSize = r.Continuity().Gap.Size
	}
	w := relationshipWire{
		ID:         r.ID().String(),
		From:       r.From().String(),
		To:         r.To().String(),
		Type:       int(r.Type()),
		Certainty:  int(r.Certainty()),
		Continuity: cont,
		Notes:      r.Notes(),
	}
	if !r.TimeRange().IsZero() {
		w.TimeRange = encodeTimeRange(r.TimeRange())
	}
	for _, sid := range r.Sources() {
		w.Sources = append(w.Sources, sid.String())
	}
	return json.Marshal(w)
}

func decodeRelationship(buf []byte) (model.Relationship, error) {
	var w relationshipWire
	if err := json.Unmarshal(buf, &w); err != nil {
		return model.Relationship{}, fmt.Errorf("decode relationship: %w", err)
	}
	id, err := model.ParseID(w.ID)
	if err != nil {
		return model.Relationship{}, err
	}
	from, err := model.ParseID(w.From)
	if err != nil {
		return model.Relationship{}, err
	}
	to, err := model.ParseID(w.To)
	if err != nil {
		return model.Relationship{}, err
	}
	contState, err := safeUint8(w.Continuity.State)
	if err != nil {
		return model.Relationship{}, fmt.Errorf("decode relationship %s continuity: %w", id, err)
	}
	var cont model.Continuity
	switch model.ContinuityState(contState) {
	case model.ContinuityContinuous:
		cont = model.NewContinuous()
	case model.ContinuityGapped:
		gap := model.UnknownGap()
		if w.Continuity.GapKnown {
			g, err := model.KnownGap(w.Continuity.GapSize)
			if err != nil {
				return model.Relationship{}, fmt.Errorf("decode relationship %s: %w", id, err)
			}
			gap = g
		}
		cont = model.NewGapped(gap)
	default:
		return model.Relationship{}, fmt.Errorf(
			"decode relationship %s: invalid continuity state %d", id, w.Continuity.State)
	}
	opts := model.RelationshipOptions{
		Notes: w.Notes,
	}
	if w.TimeRange != nil {
		opts.TimeRange = decodeTimeRange(w.TimeRange)
	}
	for _, s := range w.Sources {
		sid, err := model.ParseID(s)
		if err != nil {
			return model.Relationship{}, err
		}
		opts.Sources = append(opts.Sources, sid)
	}
	rtVal, err := safeUint8(w.Type)
	if err != nil {
		return model.Relationship{}, fmt.Errorf("decode relationship %s type: %w", id, err)
	}
	cVal, err := safeUint8(w.Certainty)
	if err != nil {
		return model.Relationship{}, fmt.Errorf("decode relationship %s certainty: %w", id, err)
	}
	return model.NewRelationship(id, from, to,
		model.RelationshipType(rtVal), model.Certainty(cVal), cont, opts)
}

// safeUint8 narrows a wire int into a uint8 used by the model
// enums, refusing values outside the uint8 range.
func safeUint8(v int) (uint8, error) {
	if v < 0 || v > 255 {
		return 0, fmt.Errorf("value %d out of uint8 range", v)
	}
	return uint8(v), nil
}

// ---------- Source ----------

type sourceWire struct {
	ID       string `json:"id"`
	Type     string `json:"type,omitempty"`
	Citation string `json:"citation"`
	Author   string `json:"author,omitempty"`
	Title    string `json:"title,omitempty"`
	Date     string `json:"date,omitempty"`
	Locator  string `json:"locator,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

func encodeSource(s model.Source) ([]byte, error) {
	return json.Marshal(sourceWire{
		ID:       s.ID().String(),
		Type:     string(s.Type()),
		Citation: s.Citation(),
		Author:   s.Author(),
		Title:    s.Title(),
		Date:     s.Date(),
		Locator:  s.Locator(),
		Notes:    s.Notes(),
	})
}

func decodeSource(buf []byte) (model.Source, error) {
	var w sourceWire
	if err := json.Unmarshal(buf, &w); err != nil {
		return model.Source{}, fmt.Errorf("decode source: %w", err)
	}
	id, err := model.ParseID(w.ID)
	if err != nil {
		return model.Source{}, err
	}
	return model.NewSource(id, model.SourceType(w.Type), w.Citation, model.SourceOptions{
		Author:  w.Author,
		Title:   w.Title,
		Date:    w.Date,
		Locator: w.Locator,
		Notes:   w.Notes,
	})
}

// ---------- Proposal ----------

type proposalWire struct {
	ID          string                `json:"id"`
	State       int                   `json:"state"`
	Action      int                   `json:"action"`
	EntityKind  int                   `json:"kind"`
	TargetID    string                `json:"target,omitempty"`
	SecondaryID string                `json:"secondary,omitempty"`
	Payload     []byte                `json:"payload,omitempty"`
	Reason      string                `json:"reason,omitempty"`
	Sources     []string              `json:"sources,omitempty"`
	Author      string                `json:"author,omitempty"`
	CreatedAt   int64                 `json:"created,omitempty"`
	History     []proposalTransWire   `json:"history,omitempty"`
}

type proposalTransWire struct {
	From      int    `json:"from"`
	To        int    `json:"to"`
	Actor     string `json:"actor,omitempty"`
	Timestamp int64  `json:"ts,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

func encodeProposal(p model.Proposal) ([]byte, error) {
	w := proposalWire{
		ID:          p.ID().String(),
		State:       int(p.State()),
		Action:      int(p.Action()),
		EntityKind:  int(p.EntityKind()),
		TargetID:    p.TargetID().String(),
		SecondaryID: p.SecondaryID().String(),
		Payload:     p.Payload(),
		Reason:      p.Reason(),
		Author:      p.Author(),
		CreatedAt:   p.CreatedAt(),
	}
	for _, sid := range p.Sources() {
		w.Sources = append(w.Sources, sid.String())
	}
	for _, h := range p.History() {
		w.History = append(w.History, proposalTransWire{
			From:      int(h.From),
			To:        int(h.To),
			Actor:     h.Actor,
			Timestamp: h.Timestamp,
			Reason:    h.Reason,
		})
	}
	return json.Marshal(w)
}

func decodeProposal(buf []byte) (model.Proposal, error) {
	var w proposalWire
	if err := json.Unmarshal(buf, &w); err != nil {
		return model.Proposal{}, fmt.Errorf("decode proposal: %w", err)
	}
	id, err := model.ParseID(w.ID)
	if err != nil {
		return model.Proposal{}, err
	}
	opts := model.ProposalOptions{
		Payload:   w.Payload,
		Reason:    w.Reason,
		Author:    w.Author,
		CreatedAt: w.CreatedAt,
	}
	if w.TargetID != "" {
		tid, err := model.ParseID(w.TargetID)
		if err != nil {
			return model.Proposal{}, err
		}
		opts.TargetID = tid
	}
	if w.SecondaryID != "" {
		sid, err := model.ParseID(w.SecondaryID)
		if err != nil {
			return model.Proposal{}, err
		}
		opts.SecondaryID = sid
	}
	for _, s := range w.Sources {
		sid, err := model.ParseID(s)
		if err != nil {
			return model.Proposal{}, err
		}
		opts.Sources = append(opts.Sources, sid)
	}
	actionVal, err := safeUint8(w.Action)
	if err != nil {
		return model.Proposal{}, fmt.Errorf("decode proposal %s action: %w", id, err)
	}
	kindVal, err := safeUint8(w.EntityKind)
	if err != nil {
		return model.Proposal{}, fmt.Errorf("decode proposal %s entity kind: %w", id, err)
	}
	p, err := model.NewProposal(id,
		model.ProposalAction(actionVal), model.EntityKind(kindVal), opts)
	if err != nil {
		return model.Proposal{}, err
	}
	// Replay history to reach the persisted state.
	for _, h := range w.History {
		fromVal, err := safeUint8(h.From)
		if err != nil {
			return model.Proposal{}, fmt.Errorf("decode proposal %s history.from: %w", id, err)
		}
		toVal, err := safeUint8(h.To)
		if err != nil {
			return model.Proposal{}, fmt.Errorf("decode proposal %s history.to: %w", id, err)
		}
		p = p.WithStateUnchecked(model.ProposalState(toVal), model.ProposalTransition{
			From:      model.ProposalState(fromVal),
			To:        model.ProposalState(toVal),
			Actor:     h.Actor,
			Timestamp: h.Timestamp,
			Reason:    h.Reason,
		})
	}
	// If state on disk doesn't match replay outcome, force-set the
	// persisted state via a no-op transition. This handles the
	// (rare) case where state was mutated without a matching
	// history entry — we trust persisted state as the source of
	// truth.
	if int(p.State()) != w.State {
		stateVal, err := safeUint8(w.State)
		if err != nil {
			return model.Proposal{}, fmt.Errorf("decode proposal %s state: %w", id, err)
		}
		p = p.WithStateUnchecked(model.ProposalState(stateVal), model.ProposalTransition{
			From: p.State(),
			To:   model.ProposalState(stateVal),
		})
	}
	return p, nil
}
