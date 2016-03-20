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

package cfgpath_test

import (
	"errors"
	"hash/fnv"
	"strconv"
	"testing"

	"github.com/corestoreio/csfw/config/cfgpath"
	"github.com/corestoreio/csfw/storage/text"
	"github.com/corestoreio/csfw/store/scope"
	"github.com/stretchr/testify/assert"
)

func TestNewByParts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		parts   []string
		want    string
		wantErr error
	}{
		{[]string{"aa/bb/cc"}, "aa/bb/cc", nil},
		{[]string{"aa/bb", "cc"}, "aa/bb/cc", nil},
		{[]string{"aa", "bb", "cc"}, "aa/bb/cc", nil},
		{[]string{"aa", "bb", "c"}, "aa/bb/cc", cfgpath.ErrIncorrectPath},
		{nil, "", cfgpath.ErrRouteEmpty},
		{[]string{""}, "", cfgpath.ErrRouteEmpty},
	}
	for i, test := range tests {
		haveP, haveErr := cfgpath.NewByParts(test.parts...)
		if test.wantErr != nil {
			assert.Nil(t, haveP.Route.Chars, "Index %d", i)
			assert.EqualError(t, haveErr, test.wantErr.Error(), "Index %d", i)
			continue
		}
		l, err := haveP.Level(-1)
		assert.NoError(t, err, "Index %d", i)
		assert.Exactly(t, test.want, l.String(), "Index %d", i)
	}
}

func TestMustNewByPartsPanic(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r != nil {
			assert.EqualError(t, r.(error), cfgpath.ErrRouteInvalidBytes.Error())
		} else {
			t.Fatal("Expecting a panic")
		}
	}()
	_ = cfgpath.MustNewByParts("a/\x80/c")
}

func TestMustNewByPartsNoPanic(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r != nil {
			assert.NotNil(t, r, "Did not expect a panic")
		} else {
			assert.Nil(t, r, "Why is here a panic")
		}
	}()
	p := cfgpath.MustNewByParts("aa", "bb", "cc")
	assert.Exactly(t, "default/0/aa/bb/cc", p.String())
}

var benchmarkNewByParts cfgpath.Path

// BenchmarkNewByParts-4	 5000000	       297 ns/op	      48 B/op	       1 allocs/op
func BenchmarkNewByParts(b *testing.B) {
	want := cfgpath.NewRoute("general/single_store_mode/enabled")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		benchmarkNewByParts, err = cfgpath.NewByParts("general", "single_store_mode", "enabled")
		if err != nil {
			b.Error(err)
		}
	}
	if benchmarkNewByParts.Route.Equal(want) == false {
		b.Errorf("Want: %s; Have, %s", want, benchmarkNewByParts.Route)
	}
}

func TestPathNewSum32(t *testing.T) {
	t.Parallel()
	r := cfgpath.Route{
		Chars: text.Chars(`dd/ee/ff`),
	}
	h := fnv.New32a()
	_, err := h.Write(r.Chars)
	assert.NoError(t, err)
	wantHash := h.Sum32()

	p, err := cfgpath.New(r)
	assert.NoError(t, err)
	assert.Exactly(t, wantHash, p.Sum32)
}

func TestPathNew(t *testing.T) {
	t.Parallel()
	tests := []struct {
		route      cfgpath.Route
		s          scope.Scope
		id         int64
		wantFQ     cfgpath.Route
		wantNewErr error
	}{
		{cfgpath.NewRoute("ab/b\x80/cd"), scope.WebsiteID, 3, cfgpath.NewRoute("websites/3/ab/ba/cd"), cfgpath.ErrRouteInvalidBytes},
		{cfgpath.NewRoute("ab/ba/cd"), scope.WebsiteID, 3, cfgpath.NewRoute("websites/3/ab/ba/cd"), nil},
		{cfgpath.NewRoute("ad/ba/ca/sd"), scope.WebsiteID, 3, cfgpath.NewRoute("websites/3/ad/ba/ca/sd"), nil},
		{cfgpath.NewRoute("as/sb"), scope.WebsiteID, 3, cfgpath.NewRoute("websites/3/a/b/c/d"), cfgpath.ErrIncorrectPath},
		{cfgpath.NewRoute("aa/bb/cc"), scope.GroupID, 3, cfgpath.NewRoute("default/0/aa/bb/cc"), nil},
		{cfgpath.NewRoute("aa/bb/cc"), scope.StoreID, 3, cfgpath.NewRoute("stores/3/aa/bb/cc"), nil},
	}
	for i, test := range tests {
		haveP, haveErr := cfgpath.New(test.route)
		haveP = haveP.Bind(test.s, test.id)
		if test.wantNewErr != nil {
			assert.EqualError(t, haveErr, test.wantNewErr.Error(), "Index %d", i)
			continue
		}
		fq, fqErr := haveP.FQ()
		assert.NoError(t, fqErr, "Index %d", i)
		assert.Exactly(t, test.wantFQ, fq, "Index %d", i)
	}
}

func TestFQ(t *testing.T) {
	t.Parallel()
	tests := []struct {
		str     scope.StrScope
		id      int64
		route   cfgpath.Route
		want    string
		wantErr error
	}{
		{scope.StrDefault, 0, cfgpath.Route{}, "", cfgpath.ErrRouteEmpty},
		{scope.StrDefault, 0, cfgpath.NewRoute(""), "", cfgpath.ErrRouteEmpty},
		{scope.StrDefault, 0, cfgpath.NewRoute("system/dev/debug"), scope.StrDefault.String() + "/0/system/dev/debug", nil},
		{scope.StrDefault, 44, cfgpath.NewRoute("system/dev/debug"), scope.StrDefault.String() + "/0/system/dev/debug", nil},
		{scope.StrWebsites, 0, cfgpath.NewRoute("system/dev/debug"), scope.StrWebsites.String() + "/0/system/dev/debug", nil},
		{scope.StrWebsites, 343, cfgpath.NewRoute("system/dev/debug"), scope.StrWebsites.String() + "/343/system/dev/debug", nil},
		{scope.StrScope("hello"), 0, cfgpath.NewRoute("system/dev/debug"), scope.StrDefault.String() + "/0/system/dev/debug", nil},
		{scope.StrScope("hello"), 343, cfgpath.NewRoute("system/dev/debug"), scope.StrDefault.String() + "/0/system/dev/debug", nil},
	}
	for i, test := range tests {
		p, pErr := cfgpath.New(test.route)
		p = p.BindStr(test.str, test.id)
		have, haveErr := p.FQ()
		if test.wantErr != nil {
			assert.Empty(t, have.Chars, "Index %d", i)
			if pErr != nil {
				assert.EqualError(t, pErr, test.wantErr.Error(), "Index %d", i)
				continue
			}
			assert.EqualError(t, haveErr, test.wantErr.Error(), "Index %d", i)
			continue
		}
		assert.NoError(t, haveErr, "Index %d", i)
		assert.Exactly(t, test.want, have.String(), "Index %d", i)
	}

	r := cfgpath.NewRoute("catalog/frontend/list_allow_all")
	assert.Exactly(t, "stores/7475/catalog/frontend/list_allow_all", cfgpath.MustNew(r).BindStr(scope.StrStores, 7475).String())
	p := cfgpath.MustNew(r).BindStr(scope.StrStores, 5)
	assert.Exactly(t, "stores/5/catalog/frontend/list_allow_all", p.String())
	assert.Exactly(t, "cfgpath.Path{ Route:cfgpath.NewRoute(`catalog/frontend/list_allow_all`), Scope: 4, ID: 5 }", p.GoString())
}

func TestShouldNotPanicBecauseOfIncorrectStrScope(t *testing.T) {
	t.Parallel()
	assert.Exactly(t, "stores/345/xxxxx/yyyyy/zzzzz", cfgpath.MustNew(cfgpath.NewRoute("xxxxx/yyyyy/zzzzz")).BindStr(scope.StrStores, 345).String())
	defer func() {
		if r := recover(); r != nil {
			t.Fatal("Did not expect a panic")
		}
	}()
	_ = cfgpath.MustNew(cfgpath.NewRoute("xxxxx/yyyyy/zzzzz")).BindStr(scope.StrScope("invalid"), 345)
}

func TestShouldPanicIncorrectPath(t *testing.T) {
	t.Parallel()
	assert.Exactly(t, "default/0/xxxxx/yyyyy/zzzzz", cfgpath.MustNew(cfgpath.NewRoute("xxxxx/yyyyy/zzzzz")).BindStr(scope.StrDefault, 345).String())
	defer func() {
		if r := recover(); r != nil {
			assert.EqualError(t, r.(error), cfgpath.ErrIncorrectPath.Error())
		} else {
			t.Fatal("Expecting a panic")
		}
	}()
	assert.Exactly(t, "websites/345/xxxxx/yyyyy", cfgpath.MustNew(cfgpath.NewRoute("xxxxx/yyyyy")).BindStr(scope.StrWebsites, 345).String())
}

var benchmarkPathFQ cfgpath.Route

// BenchmarkPathFQ-4     	 3000000	       401 ns/op	     112 B/op	       1 allocs/op
func BenchmarkPathFQ(b *testing.B) {
	var scopeID int64 = 11
	want := cfgpath.NewRoute(scope.StrWebsites.String() + "/" + strconv.FormatInt(scopeID, 10) + "/system/dev/debug")
	p := cfgpath.NewRoute("system/dev/debug")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		benchmarkPathFQ, err = cfgpath.MustNew(p).BindStr(scope.StrWebsites, scopeID).FQ()
		if err != nil {
			b.Error(err)
		}
	}
	if benchmarkPathFQ.Equal(want) == false {
		b.Errorf("Want: %s; Have, %s", want, benchmarkPathFQ)
	}
}

var benchmarkPathHash uint32

// BenchmarkPathHashFull-4  	 3000000	       502 ns/op	       0 B/op	       0 allocs/op
func BenchmarkPathHashFull(b *testing.B) {
	const scopeID int64 = 12
	const want uint32 = 1479679325
	p := cfgpath.NewRoute("system/dev/debug")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		benchmarkPathHash, err = cfgpath.MustNew(p).BindStr(scope.StrWebsites, scopeID).Hash(-1)
		if err != nil {
			b.Error(err)
		}
	}
	if benchmarkPathHash != want {
		b.Errorf("Want: %d; Have, %d", want, benchmarkPathHash)
	}
}

func BenchmarkPathHashLevel2(b *testing.B) {
	const scopeID int64 = 13
	const want uint32 = 723768876
	p := cfgpath.NewRoute("system/dev/debug")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		benchmarkPathHash, err = cfgpath.MustNew(p).BindStr(scope.StrWebsites, scopeID).Hash(2)
		if err != nil {
			b.Error(err)
		}
	}
	if benchmarkPathHash != want {
		b.Errorf("Want: %d; Have, %d", want, benchmarkPathHash)
	}
}

func TestSplitFQ(t *testing.T) {
	t.Parallel()
	tests := []struct {
		have        string
		wantScope   string
		wantScopeID int64
		wantPath    string
		wantErr     error
	}{
		{"groups/1/catalog/frontend/list_allow_all", "default", 0, "", scope.ErrUnsupportedScope},
		{"stores/7475/catalog/frontend/list_allow_all", scope.StrStores.String(), 7475, "catalog/frontend/list_allow_all", nil},
		{"stores/4/system/full_page_cache/varnish/backend_port", scope.StrStores.String(), 4, "system/full_page_cache/varnish/backend_port", nil},
		{"websites/1/catalog/frontend/list_allow_all", scope.StrWebsites.String(), 1, "catalog/frontend/list_allow_all", nil},
		{"default/0/catalog/frontend/list_allow_all", scope.StrDefault.String(), 0, "catalog/frontend/list_allow_all", nil},
		{"default//catalog/frontend/list_allow_all", scope.StrDefault.String(), 0, "catalog/frontend/list_allow_all", errors.New("strconv.ParseInt: parsing \"\\uf8ff\": invalid syntax")},
		{"stores/123/catalog/index", "default", 0, "", errors.New("Incorrect fully qualified path: \"stores/123/catalog/index\". Expecting: strScope/ID/stores/123/catalog/index")},
	}
	for i, test := range tests {
		havePath, haveErr := cfgpath.SplitFQ(test.have)

		if test.wantErr != nil {
			assert.EqualError(t, haveErr, test.wantErr.Error(), "Index %d", i)
		} else {
			assert.NoError(t, haveErr, "Test %v", test)
		}
		assert.Exactly(t, test.wantScope, havePath.StrScope(), "Index %d", i)
		assert.Exactly(t, test.wantScopeID, havePath.ID, "Index %d", i)
		l, _ := havePath.Level(-1)
		assert.Exactly(t, test.wantPath, l.String(), "Index %d", i)
	}
}

var benchmarkReverseFQPath cfgpath.Path

// BenchmarkSplitFQ-4  	10000000	       199 ns/op	      32 B/op	       1 allocs/op
func BenchmarkSplitFQ(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var err error
		benchmarkReverseFQPath, err = cfgpath.SplitFQ("stores/7475/catalog/frontend/list_allow_all")
		if err != nil {
			b.Error(err)
		}
	}
	l, _ := benchmarkReverseFQPath.Level(-1)
	if l.String() != "catalog/frontend/list_allow_all" {
		b.Error("catalog/frontend/list_allow_all not found in Level()")
	}
}

func TestPathIsValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		s    scope.Scope
		id   int64
		have cfgpath.Route
		want error
	}{
		{scope.DefaultID, 0, cfgpath.NewRoute("//"), cfgpath.ErrIncorrectPath},
		{scope.DefaultID, 0, cfgpath.NewRoute("general/store_information/city"), nil},
		{scope.DefaultID, 33, cfgpath.NewRoute("general/store_information/city"), nil},
		{scope.WebsiteID, 33, cfgpath.NewRoute("system/full_page_cache/varnish/backend_port"), nil},
		{scope.DefaultID, 0, cfgpath.NewRoute(""), cfgpath.ErrRouteEmpty},
		{scope.DefaultID, 0, cfgpath.NewRoute("general/store_information"), cfgpath.ErrIncorrectPath},
		////{cfgpath.NewRoute(cfgpath.MustNew("system/dev/debug").Bind(scope.WebsiteID, 22).String()), cfgpath.ErrIncorrectPath},
		{scope.DefaultID, 0, cfgpath.NewRoute("groups/33/general/store_information/street"), nil},
		{scope.DefaultID, 0, cfgpath.NewRoute("groups/33"), cfgpath.ErrIncorrectPath},
		{scope.DefaultID, 0, cfgpath.NewRoute("system/dEv/inv˚lid"), errors.New("This character \"˚\" is not allowed in Route system/dEv/inv˚lid")},
		{scope.DefaultID, 0, cfgpath.NewRoute("system/dEv/inv'lid"), errors.New("This character \"'\" is not allowed in Route system/dEv/inv'lid")},
		{scope.DefaultID, 0, cfgpath.NewRoute("syst3m/dEv/invalid"), nil},
		{scope.DefaultID, 0, cfgpath.Route{}, cfgpath.ErrRouteEmpty},
	}
	for i, test := range tests {
		p := cfgpath.Path{
			Scope: test.s,
			ID:    test.id,
			Route: test.have,
		}
		haveErr := p.IsValid()
		if test.want != nil {
			assert.EqualError(t, haveErr, test.want.Error(), "Index %d", i)
		} else {
			assert.NoError(t, haveErr, "Index %d", i)
		}
	}
}

func TestPathRouteIsValid(t *testing.T) {
	t.Parallel()
	p := cfgpath.Path{
		Scope: scope.StoreID,
		ID:    2,
		Route: cfgpath.NewRoute(`general/store_information`),
	}
	assert.EqualError(t, p.IsValid(), cfgpath.ErrIncorrectPath.Error())

	p = cfgpath.Path{
		Scope:           scope.StoreID,
		ID:              2,
		Route:           cfgpath.NewRoute(`general/store_information`),
		RouteLevelValid: true,
	}
	assert.NoError(t, p.IsValid())
}

func TestPathHashWebsite(t *testing.T) {
	t.Parallel()

	p := cfgpath.MustNewByParts("general/single_store_mode/enabled").Bind(scope.WebsiteID, 33)
	hv, err := p.Hash(-1)
	if err != nil {
		t.Fatal(err)
	}

	check := fnv.New32a()
	_, cErr := check.Write([]byte(p.String()))
	assert.NoError(t, cErr)
	assert.Exactly(t, check.Sum32(), hv, "Have %d want %d", hv, check.Sum32())

}

func TestPathHashDefault(t *testing.T) {
	t.Parallel()
	tests := []struct {
		have      cfgpath.Route
		level     int
		wantHash  uint32
		wantErr   error
		wantLevel string
	}{
		{cfgpath.NewRoute("general/single_\x80store_mode/enabled"), 0, 0, cfgpath.ErrRouteInvalidBytes, ""},
		{cfgpath.NewRoute("general/single_store_mode/enabled"), 0, 453736105, nil, "default/0"},
		{cfgpath.NewRoute("general/single_store_mode/enabled"), 1, 2243014074, nil, "default/0/general"},
		{cfgpath.NewRoute("general/single_store_mode/enabled"), 2, 4182795913, nil, "default/0/general/single_store_mode"},
		{cfgpath.NewRoute("general/single_store_mode/enabled"), 3, 1584651487, nil, "default/0/general/single_store_mode/enabled"},
		{cfgpath.NewRoute("general/single_store_mode/enabled"), -1, 1584651487, nil, "default/0/general/single_store_mode/enabled"}, // 5
		{cfgpath.NewRoute("general/single_store_mode/enabled"), 5, 1584651487, nil, "default/0/general/single_store_mode/enabled"},  // 6
		{cfgpath.NewRoute("general/single_store_mode/enabled"), 4, 1584651487, nil, "default/0/general/single_store_mode/enabled"},  // 7
	}
	for i, test := range tests {
		p := cfgpath.Path{
			Route: test.have,
		}

		hv, err := p.Hash(test.level)
		if test.wantErr != nil {
			assert.EqualError(t, err, test.wantErr.Error(), "Index %d", i)
			assert.Empty(t, hv, "Index %d", i)
			continue
		}
		assert.NoError(t, err, "Index %d", i)

		check := fnv.New32a()
		_, cErr := check.Write([]byte(test.wantLevel))
		assert.NoError(t, cErr)
		assert.Exactly(t, check.Sum32(), hv, "Want %d Have %d Index %d", check.Sum32(), hv, i)

		xr, err := p.FQ()
		if err != nil {
			t.Fatal(err)
		}

		if test.level < 0 {
			test.level = -3
		}
		xrl, err := xr.Level(test.level + 2)
		if err != nil {
			t.Fatal(err)
		}
		assert.Exactly(t, test.wantLevel, xrl.String(), "Index %d", i)
		assert.Exactly(t, test.wantHash, hv, "Want %d Have %d Index %d", test.wantHash, hv, i)
	}
}

func TestPathPartPosition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		have     cfgpath.Route
		level    int
		wantPart string
		wantErr  error
	}{
		{cfgpath.NewRoute("general/single_\x80store_mode/enabled"), 0, "", cfgpath.ErrIncorrectPosition},
		{cfgpath.NewRoute("general/single_store_mode/enabled"), 0, "", cfgpath.ErrIncorrectPosition},
		{cfgpath.NewRoute("general/single_store_mode/enabled"), 1, "general", nil},
		{cfgpath.NewRoute("general/single_store_mode/enabled"), 2, "single_store_mode", nil},
		{cfgpath.NewRoute("general/single_store_mode/enabled"), 3, "enabled", nil},
		{cfgpath.NewRoute("general/single_store_mode/enabled"), -1, "", cfgpath.ErrIncorrectPosition},
		{cfgpath.NewRoute("general/single_store_mode/enabled"), 5, "", cfgpath.ErrIncorrectPosition},
		{cfgpath.NewRoute("general/single/store/website/group/mode/enabled/disabled/default"), 5, "group", nil},
		{cfgpath.NewRoute("system/full_page_cache/varnish/backend_port"), 3, "varnish", nil},
		{cfgpath.NewRoute("system/full_page_cache/varnish/backend_port"), 4, "backend_port", nil},
	}
	for i, test := range tests {
		p := cfgpath.Path{
			Route: test.have,
		}
		part, haveErr := p.Part(test.level)
		if test.wantErr != nil {
			assert.EqualError(t, haveErr, test.wantErr.Error(), "Index %d", i)
			assert.Nil(t, part.Chars, "Index %d", i)
			continue
		}
		assert.Exactly(t, test.wantPart, part.String(), "Index %d", i)
	}
}

func TestPathCloneRareUseCase(t *testing.T) {
	t.Parallel()
	rs := "aa/bb/cc"
	pOrg := cfgpath.MustNewByParts(rs)
	pOrg = pOrg.Bind(scope.StoreID, 3141)

	largerBuff := make(text.Chars, 100, 100)
	pOrg.Chars = largerBuff[:copy(largerBuff, rs)]

	pAssigned := pOrg
	pCloned := pOrg.Clone()

	assert.Exactly(t, pOrg.Scope, pCloned.Scope)
	assert.Exactly(t, pOrg.ID, pCloned.ID)
	assert.Exactly(t, pOrg.Route, pCloned.Route)

	assert.Exactly(t, pOrg.Scope, pAssigned.Scope)
	assert.Exactly(t, pOrg.ID, pAssigned.ID)
	assert.Exactly(t, pOrg.Route, pAssigned.Route)

	// we're not using Path.Append because it creates internally a new byte slice
	// this append() grows the slice without creating a new one because the cap == 100, see above.
	pOrg.Chars = append(pOrg.Chars, []byte(`/dd`)...)

	assert.Exactly(t, "stores/3141/"+rs+"/dd", pOrg.String())

	assert.Exactly(t, "stores/3141/"+rs, pAssigned.String())

	assert.NotEqual(t, pOrg, pAssigned)

	// now expand the slice
	pAssigned.Chars = pAssigned.Chars[:len(pOrg.Chars)]
	assert.Exactly(t, "stores/3141/"+rs+"/dd", pAssigned.String())
	assert.Exactly(t, pOrg, pAssigned)
	assert.Exactly(t, "stores/3141/"+rs, pCloned.String())
	assert.NotEqual(t, pOrg, pCloned)
}

func TestPathCloneAppend(t *testing.T) {
	t.Parallel()
	rs := "aa/bb/cc"
	pOrg := cfgpath.MustNewByParts(rs)
	pOrg = pOrg.Bind(scope.StoreID, 3141)

	pAssigned := pOrg
	assert.Exactly(t, pOrg, pAssigned)
	assert.NoError(t, pOrg.Append(cfgpath.NewRoute("dd")))
	assert.NotEqual(t, pOrg, pAssigned)
}