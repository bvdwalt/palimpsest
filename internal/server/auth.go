package server

import (
	"crypto/subtle"
	"net/http"

	"github.com/bvdwalt/palimpsest/internal/store"
)

// requireAPIKey rejects requests whose X-Api-Key header doesn't match the
// store's current API key. Not a real access boundary (see the settings UI
// caveat), but it keeps naive callers off the API and gives external
// integrations a revocable credential.
func requireAPIKey(st *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key, err := st.GetOrCreateAPIKey(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}

			got := r.Header.Get("X-Api-Key")
			if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(key)) != 1 {
				writeError(w, http.StatusUnauthorized, "missing or invalid API key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
