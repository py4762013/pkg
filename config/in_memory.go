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

package config

import (
	"sync"

	"github.com/corestoreio/pkg/config/cfgpath"
	"github.com/corestoreio/errors"
)

type keyVal struct {
	k cfgpath.Path
	v interface{}
}

type kvmap struct {
	sync.RWMutex
	kv map[uint32]keyVal // todo: create benchmark to check if worth switching to pointers
}

// NewInMemoryStore creates a new simple key value storage using a map[string]interface{}.
// Mainly used for testing.
func NewInMemoryStore() Storager {
	return &kvmap{
		kv: make(map[uint32]keyVal),
	}
}

// Set implements Storager interface
func (sp *kvmap) Set(key cfgpath.Path, value interface{}) error {
	h32, err := key.Hash(-1)
	if err != nil {
		return errors.Wrap(err, "[storage] key.Hash")
	}
	sp.Lock()
	sp.kv[h32] = keyVal{key, value}
	sp.Unlock()
	return nil
}

// Get implements Storager interface.
// Error behaviour: NotFound.
func (sp *kvmap) Get(key cfgpath.Path) (interface{}, error) {
	h32, err := key.Hash(-1)
	if err != nil {
		return nil, errors.Wrap(err, "[storage] key.Hash")
	}
	sp.RLock()
	data, ok := sp.kv[h32]
	sp.RUnlock()
	if ok {
		return data.v, nil
	}
	return nil, keyNotFound{key}
}

// AllKeys implements Storager interface
func (sp *kvmap) AllKeys() (cfgpath.PathSlice, error) {
	sp.RLock()

	var ret = make(cfgpath.PathSlice, len(sp.kv))
	i := 0
	for _, kv := range sp.kv {
		ret[i] = kv.k
		i++
	}
	sp.RUnlock()
	return ret, nil
}

// keyNotFound for performance and allocs reasons in benchmarks to test properly
// the cfg* code and not the configuration Service. The NotFound error has been
// hard coded which does not record the position where the error happens. We can
// maybe add the path which was not found but that will trigger 2 allocs because
// of the sprintf ... which could be bypassed with a bufferpool ;-)
type keyNotFound struct{ key cfgpath.Path }

func (a keyNotFound) Error() string  { return "[config] KVMap Unknown Key: " + a.key.String() }
func (a keyNotFound) NotFound() bool { return true }
