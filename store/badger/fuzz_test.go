// SPDX-License-Identifier: AGPL-3.0-or-later

package badger

import "testing"

// FuzzDecodePerson exercises the Person JSON decoder with arbitrary
// bytes. The decoder MUST never panic — invalid input should always
// surface as an error.
func FuzzDecodePerson(f *testing.F) {
	f.Add([]byte(`{"id":"x","names":[{"text":"a","type":"full","preferred":true}]}`))
	f.Add([]byte(`{"id":"u","unknown":true}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(``))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = decodePerson(data)
	})
}

// FuzzDecodeRelationship covers the Relationship JSON decoder.
func FuzzDecodeRelationship(f *testing.F) {
	f.Add([]byte(`{"id":"r","from":"a","to":"b","type":1,"cert":3,"cont":{"state":1}}`))
	f.Add([]byte(`{"id":"r","from":"a","to":"b","type":2,"cert":3,"cont":{"state":1}}`))
	f.Add([]byte(`{"id":"r","from":"a","to":"b","type":1,"cert":2,"cont":{"state":2,"gk":true,"gs":3}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"id":"r","cont":null}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = decodeRelationship(data)
	})
}

// FuzzDecodeSource covers the Source JSON decoder.
func FuzzDecodeSource(f *testing.F) {
	f.Add([]byte(`{"id":"s","type":"primary","citation":"x"}`))
	f.Add([]byte(`{"id":"s","citation":""}`))
	f.Add([]byte(`{"id":"","citation":"x"}`))
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = decodeSource(data)
	})
}

// FuzzDecodeProposal covers the Proposal JSON decoder, including
// the history-replay path.
func FuzzDecodeProposal(f *testing.F) {
	f.Add([]byte(`{"id":"p","state":1,"action":1,"kind":1}`))
	f.Add([]byte(`{"id":"p","state":4,"action":1,"kind":1,"history":[{"from":1,"to":2}]}`))
	f.Add([]byte(`{"id":"p","state":4,"action":4,"kind":1,"target":"a","secondary":"b"}`))
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = decodeProposal(data)
	})
}
