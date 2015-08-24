package violetear

import (
	"log"
	"net/http"
	"strings"
)

type Trie struct {
	Node     map[string]*Trie
	Handler  map[string]http.Handler
	HasRegex bool
}

func NewTrie() *Trie {
	return &Trie{
		Node:    make(map[string]*Trie),
		Handler: make(map[string]http.Handler),
	}
}

func (t *Trie) Set(path []string, handler http.HandlerFunc, method string) {

	if len(path) == 0 {
		log.Fatal("path cannot be empty")
	}

	key := path[0]
	newpath := path[1:]

	val, ok := t.Node[key]

	if !ok {
		val = NewTrie()
		t.Node[key] = val

		// check for regex ":"
		if strings.HasPrefix(key, ":") {
			t.HasRegex = true
		}
	}

	if len(newpath) == 0 {
		methods := strings.Split(method, ",")
		for _, v := range methods {
			val.Handler[strings.ToUpper(strings.TrimSpace(v))] = handler
		}
		return
	}

	val.Set(newpath, handler, method)
}

func (t *Trie) Get(path []string) (trie *Trie, p []string, leaf bool) {
	if len(path) == 0 {
		log.Fatal("path cannot be empty")
	}

	key := path[0]
	newpath := path[1:]

	if val, ok := t.Node[key]; ok {
		if len(newpath) == 0 {
			return val, path, true
		}
		return val.Get(newpath)
	}
	return t, path, false
}