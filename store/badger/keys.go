// SPDX-License-Identifier: AGPL-3.0-or-later

package badger

// Key layout for the Badger adapter.
//
// All keys use a single-byte type prefix followed by ASCII data.
// Prefixes are bytes (not characters) to give us a compact, scan-
// friendly layout while still being human-debuggable in hex tools.
//
//	Prefix  Meaning                       Key                                   Value
//	------  ----------------------------  ------------------------------------  -----------------------
//	'm'     Metadata                      m/v                                   uint64 (current version)
//	'p'     Person                        p/<id>                                Person JSON
//	'r'     Relationship                  r/<id>                                Relationship JSON
//	's'     Source                        s/<id>                                Source JSON
//	'q'     Proposal                      q/<id>                                Proposal JSON
//	'i'     Index                         i/pc/f/<parent>/<rid>  ParentChild from-index (parent->child)
//	                                      i/pc/t/<child>/<rid>   ParentChild to-index   (child->parent)
//	                                      i/m/<person>/<rid>     Marriage index (either endpoint)
//
// The version key is updated on every commit. Indices are
// maintained in the same Badger transaction as the entity write,
// so they are atomic with the data they point at.

const (
	prefixMeta     = 'm'
	prefixPerson   = 'p'
	prefixRel      = 'r'
	prefixSource   = 's'
	prefixProposal = 'q'
	prefixIndex    = 'i'

	sep = '/'
)

// metaVersionKey is the singleton key holding the current version.
var metaVersionKey = []byte{prefixMeta, sep, 'v'}

func personKey(id string) []byte   { return makeIDKey(prefixPerson, id) }
func relKey(id string) []byte      { return makeIDKey(prefixRel, id) }
func sourceKey(id string) []byte   { return makeIDKey(prefixSource, id) }
func proposalKey(id string) []byte { return makeIDKey(prefixProposal, id) }

// Index helpers.

func pcFromIndexKey(parent, rid string) []byte {
	return makeIndexKey("pc", "f", parent, rid)
}

func pcToIndexKey(child, rid string) []byte {
	return makeIndexKey("pc", "t", child, rid)
}

func marriageIndexKey(person, rid string) []byte {
	return makeIndexKey("m", "", person, rid)
}

// pcFromPrefix returns the scan prefix for all parent->child
// index entries belonging to `parent`.
func pcFromPrefix(parent string) []byte {
	return makeIndexPrefix("pc", "f", parent)
}

func pcToPrefix(child string) []byte {
	return makeIndexPrefix("pc", "t", child)
}

func marriagePrefix(person string) []byte {
	return makeIndexPrefix("m", "", person)
}

// makeIDKey builds an entity key: <prefix>/<id>
func makeIDKey(prefix byte, id string) []byte {
	out := make([]byte, 0, 2+len(id))
	out = append(out, prefix, sep)
	out = append(out, id...)
	return out
}

// makeIndexKey builds: i/<a>/<b>/<owner>/<rid>
// If b == "", the second segment is omitted: i/<a>/<owner>/<rid>
func makeIndexKey(a, b, owner, rid string) []byte {
	pfx := makeIndexPrefix(a, b, owner)
	out := make([]byte, 0, len(pfx)+1+len(rid))
	out = append(out, pfx...)
	out = append(out, sep)
	out = append(out, rid...)
	return out
}

// makeIndexPrefix builds the scan prefix without a trailing rid.
// Returns: i/<a>/<b>/<owner>  (or i/<a>/<owner> if b == "")
func makeIndexPrefix(a, b, owner string) []byte {
	size := 2 + len(a) + 1 + len(owner)
	if b != "" {
		size += len(b) + 1
	}
	out := make([]byte, 0, size)
	out = append(out, prefixIndex, sep)
	out = append(out, a...)
	out = append(out, sep)
	if b != "" {
		out = append(out, b...)
		out = append(out, sep)
	}
	out = append(out, owner...)
	return out
}
