// Copyright 2015, Cyrill @ Schumacher.fm and the CoreStore contributors
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

package ctxhttp

// Middleware is a wrapper for the ctxhttp.Handler to create middleware functions.
type Middleware func(Handler) Handler

// Chain function will iterate over all middleware, calling them one by one
// in a chained manner, returning the result of the final middleware.
// Execution of the middleware takes place in reverse order! First to be called
// handler must be added as last slice index.
func Chain(h Handler, mws ...Middleware) Handler {
	for _, mw := range mws {
		h = mw(h)
	}
	return h
}
