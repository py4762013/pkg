// Copyright 2015-2016, Cyrill @ Schumacher.fm and the CoreStore contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scope

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/corestoreio/csfw/util/bufferpool"
	"github.com/corestoreio/csfw/util/errors"
)

// MaxID maximum allowed ID from package store. Doesn't matter whether we have a
// website, group or store scope. int24 (8388607) size at the moment.
const MaxID int64 = 1<<23 - 1

// DefaultHash default Hash value for Default Scope and ID 0. Avoids typing
// 		scope.NewHash(DefaultID,0)
const DefaultHash Hash = Hash(Default)<<24 | 0

// Hash defines a merged Scope with its ID. The first 8 bit represents the
// scope: Default, Website, Group or Store. The last 24 bit represents the
// assigned ID. This ID relates to the database table in M2 to `website`,
// `store` or `store_group` and for M1 to `core_website`, `core_store` and
// `core_store_group`. The maximum ID which can be used is defined in constant
// MaxID.
type Hash uint32

// If we have need for more store IDs then we can change the underlying types here.

// String human readable output
func (h Hash) String() string {
	scp, id := h.Unpack()
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	_, _ = buf.WriteString("Scope(")
	_, _ = buf.WriteString(scp.String())
	_, _ = buf.WriteString(") ID(")
	nb := strconv.AppendInt(buf.Bytes(), id, 10)
	buf.Reset()
	_, _ = buf.Write(nb)
	_ = buf.WriteByte(')')
	return buf.String()
}

// GoString compilable representation of a hash.
func (h Hash) GoString() string {
	scp, id := h.Unpack()
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	_, _ = buf.WriteString("scope.NewHash(scope.")
	_, _ = buf.WriteString(scp.String())
	_, _ = buf.WriteString(", ")
	nb := strconv.AppendInt(buf.Bytes(), id, 10)
	buf.Reset()
	_, _ = buf.Write(nb)
	_ = buf.WriteByte(')')
	return buf.String()
}

// ToUint64 converts the hash
func (h Hash) ToUint64() uint64 {
	return uint64(h)
}

// Unpack extracts a Scope and its ID from a hash. Returned ID can be -1 when
// the Hash contains invalid data. An ID of -1 is considered an error.
func (h Hash) Unpack() (s Scope, id int64) {

	prospectS := h >> 24
	if prospectS > maxUint8 || prospectS < 0 {
		return Absent, -1
	}
	s = Scope(prospectS)

	h64 := int64(h)
	prospectID := h64 ^ (h64>>24)<<24
	if prospectID > MaxID || prospectID < 0 {
		return Absent, -1
	}

	id = prospectID
	return
}

// EqualScope compares the scope of two hashes and returns true if their scope
// matches. This functions checks overflows, would then return false. Two hashes
// with an Absent scope are never equal.
func (h Hash) EqualScope(other Hash) bool {
	hScope := h >> 24
	if hScope > maxUint8 || hScope <= 0 {
		return false
	}
	oScope := other >> 24
	if oScope > maxUint8 || oScope <= 0 {
		return false
	}
	return hScope == oScope
}

// Scope returns the underlying assigned scope.
func (h Hash) Scope() Scope {
	hScope := h >> 24
	if hScope > maxUint8 || hScope < 0 {
		return Absent
	}
	return Scope(hScope)
}

// ID returns the underlying assigned ID. If the ID overflows the MaxStoreID or
// is smaller than zero then it returns -1.
func (h Hash) ID() int64 {
	h64 := int64(h)
	prospectID := h64 ^ (h64>>24)<<24
	if prospectID > MaxID || prospectID < 0 {
		return -1
	}
	return prospectID
}

// ValidParent validates if the parent scope is within the hierarchical chain:
// default -> website -> store.
func (h Hash) ValidParent(parent Hash) bool {
	p, pID := parent.Unpack()
	c, cID := h.Unpack()
	return (p == Default && pID == 0 && c == Default && cID == 0) ||
		(p == Default && pID == 0 && c == Website && cID >= 0) ||
		(p == Website && pID >= 0 && c == Store && cID >= 0)
}

// CalculateRunMode transforms the Hash into a runMode. On an invalid Hash (the
// scope is < Website or scope > Store) it falls back to the default run mode,
// which is a zero Hash. Implements interface RunModeCalculater.
func (h Hash) CalculateRunMode(_ *http.Request) Hash {
	if s := h.Scope(); s < Website || s > Store {
		// fall back to default because only Website, Group and Store are allowed.
		h = DefaultRunMode
	}
	return h
}

// HashMaxSegments maximum supported segments or also known as shards. This
// constant can be used to create the segmented array in other packages.
const HashMaxSegments uint16 = 256

const hashBitAnd Hash = Hash(HashMaxSegments) - 1

// Segment generates an 0 < ID <= 255 from a hash. Only used within an array
// index to optimize map[] usage in high concurrent situations. Also known as
// shard. An array of N shards is created, each shard contains its own instance
// of the cache with a lock. When an item with unique key needs to be cached a
// shard for it is chosen at first by the function Segment(). After that the
// cache lock is acquired and a write to the cache takes place. Reads are
// analogue.
func (h Hash) Segment() uint8 {
	return uint8(h & hashBitAnd)
}

// NewHash creates a new merged value. An error is equal to returning 0. An
// error occurs when id is greater than MaxStoreID or smaller 0. An errors
// occurs when the Scope is Default and id anything else than 0.
func NewHash(s Scope, id int64) Hash {
	if id > MaxID || (s > Default && id < 0) {
		return 0
	}
	if s < Website {
		id = 0
	}
	return Hash(s)<<24 | Hash(id)
}

// Hashes collection of multiple Hash values.
type Hashes []Hash

// Len is part of sort.Interface.
func (h Hashes) Len() int { return len(h) }

// Swap is part of sort.Interface.
func (h Hashes) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

// Less is part of sort.Interface.
func (h Hashes) Less(i, j int) bool { return h[i] < h[j] }

// Lowest finds from hashes the common lowest scope. All scopes must have within
// their scope the same ID otherwise an error will be returned. This functions
// gets mainly used in backend* packages if several configuration paths must be
// applied to one functional option. Eg. config path A has scope Website(1) but
// config path B has scope Store(2) and config path C has scope Website(1) so
// the common valid hash resolves to Store(2). If there would be a config path
// with scope Store(3) then a NotValid error gets returned.
func (h Hashes) Lowest() (Hash, error) {
	sort.Stable(h)
	var pick = DefaultHash
	wIDs, gIDs, sIDs := float64(0), float64(0), float64(0)
	wC, gC, sC := float64(0), float64(0), float64(0)
	for _, v := range h {

		if v.Scope() > pick.Scope() {
			pick = v
		}

		switch v.Scope() {
		case Website:
			wC++
			wIDs += float64(v.ID())
		case Group:
			gC++
			gIDs += float64(v.ID())
		case Store:
			sC++
			sIDs += float64(v.ID())
		}
	}

	switch pick.Scope() {
	case Website:
		if float64(pick.ID()) != wIDs/wC {
			return 0, errors.NewNotValidf("[scope] Invalid hash: %s in slice.", pick)
		}
	case Group:
		if float64(pick.ID()) != gIDs/gC {
			return 0, errors.NewNotValidf("[scope] Invalid hash: %s in slice.", pick)
		}
	case Store:
		if float64(pick.ID()) != sIDs/sC {
			return 0, errors.NewNotValidf("[scope] Invalid hash: %s in slice.", pick)
		}
	case Default, Absent:
		// do nothing
	default:
		// todo implement scope independent solution ...
		return 0, errors.NewNotValidf("[scope] Invalid hash: %s in slice.", pick)

	}

	return pick, nil
}
