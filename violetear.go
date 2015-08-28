// HTTP router
package violetear

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

type Router struct {
	// Routes to be matched
	routes *Trie

	// dynamicRoutes map of dynamic routes and regular expresions
	dynamicRoutes dynamicSet

	// logRequests yes or no
	LogRequests bool

	// NotFoundHandler configurable http.Handler which is called when no matching
	// route is found. If it is not set, http.NotFound is used.
	NotFoundHandler http.Handler

	// NotAllowedHandler configurable http.Handler which is called when method not allowed.
	NotAllowedHandler http.Handler

	// request-id to use
	Request_ID string

	// extraHeaders adds exta headers to the response
	extraHeaders map[string]string

	// count counter for hits
	count int64

	// Function to handle panics recovered from http handlers.
	PanicHandler func(http.ResponseWriter, *http.Request)
}

var split_path_rx = regexp.MustCompile(`[^/ ]+`)

// New returns a new initialized router.
func New() *Router {
	return &Router{
		routes:        NewTrie(),
		dynamicRoutes: make(dynamicSet),
		extraHeaders:  make(map[string]string),
	}
}

// Run violetear as an HTTP server.
// The addr string takes the same format as http.ListenAndServe.
func (v *Router) Run(addr string) {
	log.Printf("Router listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, v))
}

// SetHeader adds extra headers to the response
func (v *Router) SetHeader(key, value string) {
	v.extraHeaders[key] = value
}

// HandleFunc add a route to the router (path, HandlerFunc, methods)
func (v *Router) HandleFunc(path string, handler http.HandlerFunc, http_methods ...string) error {
	path_parts := v.splitPath(path)

	// search for dynamic routes
	for _, p := range path_parts {
		if strings.HasPrefix(p, ":") {
			if _, ok := v.dynamicRoutes[p]; !ok {
				log.Fatalf("[%s] not found, need to add it using AddRegex(\"%s\", `your regex`)", p, p)
			}
		}
	}

	// if no methods, accept ALL
	methods := "ALL"
	if len(http_methods) > 0 {
		methods = http_methods[0]
	}

	log.Printf("Adding path: %s [%s]", path, methods)
	if err := v.routes.Set(path_parts, handler, methods); err != nil {
		log.Fatal(err)
	}
	return nil
}

// AddRegex adds a ":named" regular expression to the dynamicRoutes
func (v *Router) AddRegex(name string, regex string) error {
	return v.dynamicRoutes.Set(name, regex)
}

// MethodNotAllowed default handler for 405
func (v *Router) MethodNotAllowed() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w,
			http.StatusText(http.StatusMethodNotAllowed),
			http.StatusMethodNotAllowed,
		)
	})
}

// ServerHTTP dispatches the handler registered in the matched path
func (v *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	atomic.AddInt64(&v.count, 1)
	lw := NewResponseWriter(w)

	node, path, leaf, err := v.routes.Get(v.splitPath(r.URL.Path))

	if err != nil {
		log.Fatal("sss")
	}

	// checkMethod check if method is allowed or not
	checkMethod := func(node *Trie, method string) http.Handler {
		if h, ok := node.Handler[method]; ok {
			return h
		} else if h, ok := node.Handler["ALL"]; ok {
			return h
		}
		if v.NotAllowedHandler != nil {
			return v.NotAllowedHandler
		} else {
			return v.MethodNotAllowed()
		}
	}

	var match func(node *Trie, path []string, leaf bool) http.Handler

	// match find a handler for the request
	match = func(node *Trie, path []string, leaf bool) http.Handler {
		if len(node.Handler) > 0 && leaf {
			return checkMethod(node, r.Method)
		} else if node.HasRegex {
			for k, _ := range node.Node {
				if strings.HasPrefix(k, ":") {
					rx := v.dynamicRoutes[k]
					if rx.MatchString(path[0]) {
						path[0] = k
						if leaf {
							match(node, path, leaf)
						} else {
							return checkMethod(node.Node[k], r.Method)
						}
					}
				}
			}
			if node.HasCatchall {
				return checkMethod(node.Node["*"], r.Method)
			}
		} else if node.HasCatchall {
			return checkMethod(node.Node["*"], r.Method)
		}
		// NotFound
		if v.NotFoundHandler != nil {
			return v.NotFoundHandler
		}
		return http.NotFoundHandler()
	}

	// rid Set Request-ID
	rid := r.Header.Get(v.Request_ID)
	if rid != "" {
		w.Header().Set(v.Request_ID, rid)
	} else {
		rid = fmt.Sprintf("%s-%d-%d", r.Method, time.Now().UnixNano(), atomic.LoadInt64(&v.count))
		w.Header().Set("Request-ID", rid)
	}

	// set extra headers
	for k, v := range v.extraHeaders {
		w.Header().Set(k, v)
	}

	//h http.Handler
	h := match(node, path, leaf)
	h.ServeHTTP(lw, r)

	if v.LogRequests {
		log.Printf("%s [%s] %d %d %v %s", r.RemoteAddr, r.URL, lw.Status(), lw.Size(), time.Since(start), rid)
	}
	return
}

// splitPath returns an slice of the path
func (v *Router) splitPath(p string) []string {
	path_parts := split_path_rx.FindAllString(p, -1)

	// root (empty slice)
	if len(path_parts) == 0 {
		path_parts = append(path_parts, "/")
	}

	return path_parts
}
