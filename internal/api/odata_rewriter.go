package api

import (
	"net/http"
	"regexp"
	"strings"
)

var (
	indexODataRe = regexp.MustCompile(`/indexes\('([^']+)'\)`)
	docODataRe   = regexp.MustCompile(`/docs\('([^']+)'\)`)
)

func rewriteODataPath(path string) string {
	path = indexODataRe.ReplaceAllString(path, "/indexes/$1")
	path = docODataRe.ReplaceAllString(path, "/docs/$1")
	path = strings.ReplaceAll(path, "/search.stats", "/stats")
	path = strings.ReplaceAll(path, "/docs/search.index", "/docs/index")
	path = strings.ReplaceAll(path, "/docs/search.post.search", "/docs/search")
	return path
}

// ODataPathRewriter wraps an http.Handler and translates Azure SDK OData-style
// paths to the emulator's REST-style paths before routing.
//
// Azure SDKs generate paths like /indexes('name') and /docs/search.post.search,
// while the emulator routes use /indexes/name and /docs/search.
func ODataPathRewriter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = rewriteODataPath(r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
