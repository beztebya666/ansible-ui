// Package webui serves the compiled single-page app, embedded into the api
// binary so the whole front end ships in one container (no nginx).
//
// The real bundle is copied into ./dist by the Docker build before `go build`.
// A placeholder index.html is committed so local builds also succeed.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler returns an http.Handler that serves static assets and falls back to
// index.html for client-side routes (SPA).
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	index, _ := fs.ReadFile(sub, "index.html")
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			serveIndex(w, index)
			return
		}
		if f, err := sub.Open(p); err == nil {
			_ = f.Close()
			// Hashed build assets are immutable.
			if strings.HasPrefix(p, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		// Unknown path → SPA route.
		serveIndex(w, index)
	})
}

func serveIndex(w http.ResponseWriter, index []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(index)
}
