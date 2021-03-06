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

// Package cfgpath handles the configuration paths.
//
// It contains two main types: Path and Route:
//
//    +-------+ +-----+ +----------+ +-----+ +----------------+
//    |       | |     | |          | |     | |                |
//    | Scope | |  /  | | Scope ID | |  /  | | Route/to/Value |
//    |       | |     | |          | |     | |                |
//    +-------+ +-----+ +----------+ +-----+ +----------------+
//
//    +                                      +                +
//    |                                      |                |
//    | <--------------+ Path +-----------------------------> |
//    |                                      |                |
//    +                                      + <- Route ----> +
//
// Scope
//
// A scope can only be default, websites or stores. Those three strings are
// defined by constants in package store/scope.
//
// Scope ID
//
// Refers to the database auto increment ID of one of the tables core_website
// and core_store for M1 and store_website plus store for M2.
//
// Type Path
//
// A Path contains always the scope, its scope ID and the route.
// If scope and ID haven't been provided they fall back to scope "default"
// and ID zero (0).
// Configuration paths are mainly used in table core_config_data.
//
// Type Route
//
// A route contains bytes and does not know anything about a scope or an ID.
// In the majority of use cases a route contains three parts to package
// config/element types for building a hierarchical tree structure:
//    element.Section.ID / element.Group.ID / element.Field.ID
// To add little bit more confusion: A route can either be a short one
// like aa/bb/cc or a fully qualified path like
//     scope/scopeID/element.Section.ID/element.Group.ID/element.Field.ID
// But the route always consists of a minimum of three parts.
//
// A route can have only three groups of [a-zA-Z0-9_] characters
// split by '/'. The limitation to [a-zA-Z0-9_] is a M1/M2 thing and can be
// maybe later removed.
// Minimal length per part 2 characters. Case sensitive.
//
// The route parts are used as an ID in element.Section, element.Group and
// element.Field types. See package element.
package cfgpath
