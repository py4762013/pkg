// Copyright 2013 Julien Schmidt. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

// Package ctxrouter is a trie based high performance HTTP request router for net/context.
//
// The router matches incoming requests by the request method and the path.
// If a handle is registered for this path and method, the router delegates the
// request to that function.
// For the methods GET, POST, PUT, PATCH and DELETE shortcut functions exist to
// register handles, for all other methods router.Handle can be used.
//
// The registered path, against which the router matches incoming requests, can
// contain two types of parameters:
//  Syntax    Type
//  :name     named parameter
//  *name     catch-all parameter
//
// Named parameters are dynamic path segments. They match anything until the
// next '/' or the path end:
//  Path: /blog/:category/:post
//
//  Requests:
//   /blog/go/request-routers            match: category="go", post="request-routers"
//   /blog/go/request-routers/           no match, but the router would redirect
//   /blog/go/                           no match
//   /blog/go/request-routers/comments   no match
//
// Catch-all parameters match anything until the path end, including the
// directory index (the '/' before the catch-all). Since they match anything
// until the end, catch-all parameters must always be the final path element.
//  Path: /files/*filepath
//
//  Requests:
//   /files/                             match: filepath="/"
//   /files/LICENSE                      match: filepath="/LICENSE"
//   /files/templates/article.html       match: filepath="/templates/article.html"
//   /files                              no match, but the router would redirect
//
// The value of parameters is saved as a slice of the Param struct, consisting
// each of a key and a value. The slice is passed to the Handle func as a third
// parameter.
// There are two ways to retrieve the value of a parameter:
//  // by the name of the parameter
//  user := ps.ByName("user") // defined by :user or *user
//
//  // by the index of the parameter. This way you can also get the name (key)
//  thirdKey   := ps[2].Key   // the name of the 3rd parameter
//  thirdValue := ps[2].Value // the value of the 3rd parameter
package ctxrouter

import (
	"net/http"

	"github.com/corestoreio/csfw/net/ctxhttp"
	"github.com/corestoreio/csfw/utils"
	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
)

// Param is a single URL parameter, consisting of a key and a value.
type Param struct {
	Key   string
	Value string
}

// Params is a Param-slice, as returned by the router.
// The slice is ordered, the first URL parameter is also the first slice value.
// It is therefore safe to read values by the index.
type Params []Param

// ByName returns the value of the first Param which key matches the given name.
// If no matching Param is found, an empty string is returned.
func (ps Params) ByName(name string) string {
	for i := range ps {
		if ps[i].Key == name {
			return ps[i].Value
		}
	}
	return ""
}

type ctxKeyParams struct{}

// FromContextParams returns the Params slice from a context. It is guaranteed
// that the return value is non-nil.
func FromContextParams(ctx context.Context) Params {
	if p, ok := ctx.Value(ctxKeyParams{}).(Params); ok {
		return p
	}
	return Params{}
}

// WithContextParams puts Params into a context.
func WithContextParams(ctx context.Context, p Params) context.Context {
	return context.WithValue(ctx, ctxKeyParams{}, p)
}

type ctxKeyPanic struct{}

// FromContextPanic returns the value of a panic. You are responsible
// to extract the correct type from the interface{}.
func FromContextPanic(ctx context.Context) interface{} {
	return ctx.Value(ctxKeyPanic{})
}

// WithContextPanic puts a panic into a context.
func WithContextPanic(ctx context.Context, p interface{}) context.Context {
	return context.WithValue(ctx, ctxKeyPanic{}, p)
}

type webSocketKey struct{}

// FromContextWebsocket extracts a websocket connection from the context.
// Returns false even when the socket is nil. A true return value is guaranteed
// that the socket is not nil.
func FromContextWebsocket(ctx context.Context) (ws *websocket.Conn, ok bool) {
	ws, ok = ctx.Value(webSocketKey{}).(*websocket.Conn)
	if ok && ws == nil {
		ok = false
	}
	return
}

func withContextWebsocket(ctx context.Context, ws *websocket.Conn) context.Context {
	return context.WithValue(ctx, webSocketKey{}, ws)
}

// Router is a ctxhttp.Handler which can be used to dispatch requests to different
// handler functions via configurable routes
type Router struct {
	middleware ctxhttp.MiddlewareSlice
	prefix     string
	trees      map[string]*node

	// Enables automatic redirection if the current route can't be matched but a
	// handler for the path with (without) the trailing slash exists.
	// For example if /foo/ is requested but a route only exists for /foo, the
	// client is redirected to /foo with http status code 301 for GET requests
	// and 307 for all other request methods.
	RedirectTrailingSlash bool

	// If enabled, the router tries to fix the current request path, if no
	// handle is registered for it.
	// First superfluous path elements like ../ or // are removed.
	// Afterwards the router does a case-insensitive lookup of the cleaned path.
	// If a handle can be found for this route, the router makes a redirection
	// to the corrected path with status code 301 for GET requests and 307 for
	// all other request methods.
	// For example /FOO and /..//Foo could be redirected to /foo.
	// RedirectTrailingSlash is independent of this option.
	RedirectFixedPath bool

	// If enabled, the router checks if another method is allowed for the
	// current route, if the current request can not be routed.
	// If this is the case, the request is answered with 'Method Not Allowed'
	// and HTTP status code 405.
	// If no other Method is allowed, the request is delegated to the NotFound
	// handler.
	HandleMethodNotAllowed bool

	// Configurable ctxhttp.Handler which is called when no matching route is
	// found. If it is not set, http.NotFound is used.
	NotFound ctxhttp.Handler

	// Configurable ctxhttp.Handler which is called when a request
	// cannot be routed and HandleMethodNotAllowed is true.
	// If it is not set, http.Error with http.StatusMethodNotAllowed is used.
	MethodNotAllowed ctxhttp.Handler

	// Function to handle panics recovered from ctxhttp handlers.
	// It should be used to generate a error page and return the http error code
	// 500 (Internal Server Error).
	// The handler can be used to keep your server from crashing because of
	// unrecovered panics. You can extract the panic value from the context
	// via the PanicFromContext() function.
	PanicHandler ctxhttp.HandlerFunc

	// RootContext overall initial context which will be passed
	// to every handler. Default context is context.Background().
	// A nil context causes a panic.
	RootContext context.Context
}

// Make sure the Router conforms with the ctxhttp.Handler interface
var _ ctxhttp.Handler = (*Router)(nil)
var _ http.Handler = (*Router)(nil)

// New returns a new initialized Router.
// Path auto-correction, including trailing slashes, is enabled by default.
// Default Context is context.Background(). Argument cc can only be set 0 or 1.
func New(cc ...context.Context) *Router {
	rc := context.Background()
	if len(cc) == 1 && cc[0] != nil {
		rc = cc[0]
	}
	return &Router{
		RedirectTrailingSlash:  true,
		RedirectFixedPath:      true,
		HandleMethodNotAllowed: true,
		RootContext:            rc,
	}
}

// initTree prepares a tree of nodes. used in Group() and Handle() functions
func (r *Router) initTree() {
	if r.trees == nil {
		r.trees = make(map[string]*node)
	}
}

// Use applies middleware to the router
func (r *Router) Use(mws ...ctxhttp.Middleware) {
	r.middleware = append(r.middleware, mws...)
}

// Group creates a new sub router with prefix. It inherits all properties from
// the parent. Passing middleware overrides parent middleware.
func (r *Router) Group(prefix string, mws ...ctxhttp.Middleware) *Group {
	r.initTree()
	g := &Group{r: *r} // dereference it because of custom middleware and a prefix. BUT we still need the map in the group
	g.r.prefix += prefix
	if len(mws) == 0 {
		mw := make(ctxhttp.MiddlewareSlice, len(g.r.middleware))
		copy(mw, g.r.middleware)
		g.r.middleware = mw
	} else {
		g.r.middleware = nil
		g.Use(mws...)
	}
	return g
}

// GET is a shortcut for router.Handle("GET", path, handle)
func (r *Router) GET(path string, handle ctxhttp.HandlerFunc) {
	r.Handle("GET", path, handle)
}

// HEAD is a shortcut for router.Handle("HEAD", path, handle)
func (r *Router) HEAD(path string, handle ctxhttp.HandlerFunc) {
	r.Handle("HEAD", path, handle)
}

// OPTIONS is a shortcut for router.Handle("OPTIONS", path, handle)
func (r *Router) OPTIONS(path string, handle ctxhttp.HandlerFunc) {
	r.Handle("OPTIONS", path, handle)
}

// POST is a shortcut for router.Handle("POST", path, handle)
func (r *Router) POST(path string, handle ctxhttp.HandlerFunc) {
	r.Handle("POST", path, handle)
}

// PUT is a shortcut for router.Handle("PUT", path, handle)
func (r *Router) PUT(path string, handle ctxhttp.HandlerFunc) {
	r.Handle("PUT", path, handle)
}

// PATCH is a shortcut for router.Handle("PATCH", path, handle)
func (r *Router) PATCH(path string, handle ctxhttp.HandlerFunc) {
	r.Handle("PATCH", path, handle)
}

// DELETE is a shortcut for router.Handle("DELETE", path, handle)
func (r *Router) DELETE(path string, handle ctxhttp.HandlerFunc) {
	r.Handle("DELETE", path, handle)
}

// WEBSOCKET adds a WebSocket route > handler to the router. Use the helper
// function FromContextWebsocket() to extract the websocket.Conn in your HandlerFunc
// from the context.
func (r *Router) WEBSOCKET(path string, h ctxhttp.HandlerFunc) {
	r.GET(path, func(ctx context.Context, w http.ResponseWriter, r *http.Request) (err error) {
		wss := websocket.Server{
			Handler: func(ws *websocket.Conn) {
				w.WriteHeader(http.StatusSwitchingProtocols)
				err = h(withContextWebsocket(ctx, ws), w, r)
			},
		}
		wss.ServeHTTP(w, r)
		return err
	})
}

// Handle registers a new request handle with the given path and method.
//
// For GET, POST, PUT, PATCH and DELETE requests the respective shortcut
// functions can be used.
//
// This function is intended for bulk loading and to allow the usage of less
// frequently used, non-standardized or custom methods (e.g. for internal
// communication with a proxy).
func (r *Router) Handle(method, path string, handle ctxhttp.HandlerFunc) {
	if path[0] != '/' {
		panic("path must begin with '/' in path '" + path + "'")
	}
	if r.prefix != "" && r.prefix[0] != '/' {
		panic("prefix must begin with '/' in path '" + r.prefix + "'")
	}

	r.initTree()

	root := r.trees[method]
	if root == nil {
		root = new(node)
		r.trees[method] = root
	}

	root.addRoute(r.prefix+path, handle, r.middleware)
}

// Handler is an adapter which allows the usage of an ctxhttp.Handler as a
// request handle.
func (r *Router) Handler(method, path string, handler ctxhttp.Handler) {
	r.Handle(method, path,
		func(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
			return handler.ServeHTTPContext(ctx, w, req)
		},
	)
}

// HandlerFunc is an adapter which allows the usage of an ctxhttp.HandlerFunc as a
// request handle.
func (r *Router) HandlerFunc(method, path string, handler ctxhttp.HandlerFunc) {
	r.Handler(method, path, handler)
}

// ServeFiles serves files from the given file system root.
// The path must end with "/*filepath", files are then served from the local
// path /defined/root/dir/*filepath.
// For example if root is "/etc" and *filepath is "passwd", the local file
// "/etc/passwd" would be served.
// Internally a http.FileServer is used, therefore http.NotFound is used instead
// of the Router's NotFound handler.
// To use the operating system's file system implementation,
// use http.Dir:
//     router.ServeFiles("/src/*filepath", http.Dir("/var/www"))
func (r *Router) ServeFiles(path string, root http.FileSystem) {
	if len(path) < 10 || path[len(path)-10:] != "/*filepath" {
		panic("path must end with /*filepath in path '" + path + "'")
	}

	fileServer := http.FileServer(root)

	r.GET(path, func(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
		req.URL.Path = FromContextParams(ctx).ByName("filepath")
		fileServer.ServeHTTP(w, req)
		return nil
	})
}

func (r *Router) recv(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	if rcv := recover(); rcv != nil {
		if err := r.PanicHandler(WithContextPanic(ctx, rcv), w, req); err != nil {
			http.Error(w, utils.Errors(err), http.StatusInternalServerError)
		}
	}
}

// Lookup allows the manual lookup of a method + path combo.
// This is e.g. useful to build a framework around this router.
// If the path was found, it returns the handle function. Otherwise the third
// return value indicates whether a redirection to
// the same path with an extra / without the trailing slash should be performed.
func (r *Router) Lookup(method, path string) (ctxhttp.HandlerFunc, ctxhttp.MiddlewareSlice, Params, bool) {
	if root := r.trees[method]; root != nil {
		return root.getValue(path)
	}
	return nil, nil, nil, false
}

// ServeHTTP makes the router implement the http.Handler interface. Calls the
// ServeHTTPContext function with the RootContext.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if err := r.ServeHTTPContext(r.RootContext, w, req); err != nil {
		http.Error(w, utils.Errors(err), http.StatusInternalServerError)
	}
}

// ServeHTTPContext makes the router implement the ctxhttp.Handler interface.
func (r *Router) ServeHTTPContext(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	if r.PanicHandler != nil {
		defer r.recv(ctx, w, req)
	}

	if root := r.trees[req.Method]; root != nil {
		path := req.URL.Path

		if handle, mws, ps, tsr := root.getValue(path); handle != nil {
			if mws != nil {
				handle = mws.Chain(handle)
			}
			if ps != nil {
				ctx = WithContextParams(ctx, ps)
			}
			return handle(ctx, w, req)
		} else if req.Method != "CONNECT" && path != "/" {
			code := 301 // Permanent redirect, request with GET method
			if req.Method != "GET" {
				// Temporary redirect, request with same method
				// As of Go 1.3, Go does not support status code 308.
				code = 307
			}

			if tsr && r.RedirectTrailingSlash {
				if len(path) > 1 && path[len(path)-1] == '/' {
					req.URL.Path = path[:len(path)-1]
				} else {
					req.URL.Path = path + "/"
				}
				http.Redirect(w, req, req.URL.String(), code)
				return nil
			}

			// Try to fix the request path
			if r.RedirectFixedPath {
				fixedPath, found := root.findCaseInsensitivePath(
					CleanPath(path),
					r.RedirectTrailingSlash,
				)
				if found {
					req.URL.Path = string(fixedPath)
					http.Redirect(w, req, req.URL.String(), code)
					return nil
				}
			}
		}
	}

	// Handle 405
	if r.HandleMethodNotAllowed {
		for method := range r.trees {
			// Skip the requested method - we already tried this one
			if method == req.Method {
				continue
			}

			handle, _, _, _ := r.trees[method].getValue(req.URL.Path)
			if handle != nil {
				if r.MethodNotAllowed != nil {
					return r.MethodNotAllowed.ServeHTTPContext(ctx, w, req)
				}
				http.Error(w,
					http.StatusText(http.StatusMethodNotAllowed),
					http.StatusMethodNotAllowed,
				)
				return nil
			}
		}
	}

	// Handle 404
	if r.NotFound != nil {
		return r.NotFound.ServeHTTPContext(ctx, w, req)
	}
	http.NotFound(w, req)
	return nil
}
