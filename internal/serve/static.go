package serve

import (
	"net/http"
	"os"
	"path/filepath"
)

// StaticDir serves the built SPA from dir. Unknown paths fall back to
// index.html so client-side routing keeps working after reloads.
// It returns nil when the built UI is absent, e.g. during backend tests.
func StaticDir(dir string) http.Handler {
	indexPath := filepath.Join(dir, "index.html")
	if st, err := os.Stat(indexPath); err != nil || st.IsDir() {
		return nil
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mirror net/http's path cleaning to block traversal outside dir.
		clean := filepath.Clean("/" + r.URL.Path)
		st, err := os.Stat(filepath.Join(dir, clean))
		if err != nil || st.IsDir() {
			http.ServeFile(w, r, indexPath)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, clean))
	})
}
