package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bvdwalt/inkbase/internal/store"
	"github.com/go-chi/chi/v5"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func configHandler(autosaveIntervalSeconds int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]int{"autosaveIntervalSeconds": autosaveIntervalSeconds})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func handleStoreErr(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

func listPagesHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pages, err := st.ListTree(r.Context())
		if err != nil {
			handleStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, pages)
	}
}

type createPageRequest struct {
	ParentID    *string `json:"parentId"`
	Title       string  `json:"title"`
	ContentJSON string  `json:"contentJson"`
	ContentText string  `json:"contentText"`
}

func createPageHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createPageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Title == "" {
			writeError(w, http.StatusBadRequest, "title is required")
			return
		}

		page, err := st.Create(r.Context(), req.ParentID, req.Title, req.ContentJSON, req.ContentText)
		if err != nil {
			handleStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, page)
	}
}

func getPageHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		page, err := st.Get(r.Context(), id)
		if err != nil {
			handleStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, page)
	}
}

type updatePageRequest struct {
	Title       string  `json:"title"`
	ParentID    *string `json:"parentId"`
	ContentJSON string  `json:"contentJson"`
	ContentText string  `json:"contentText"`
}

func updatePageHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		var req updatePageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Title == "" {
			writeError(w, http.StatusBadRequest, "title is required")
			return
		}

		page, err := st.Update(r.Context(), id, req.Title, req.ParentID, req.ContentJSON, req.ContentText)
		if err != nil {
			if errors.Is(err, store.ErrCycle) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			handleStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, page)
	}
}

type movePageRequest struct {
	ParentID *string `json:"parentId"`
}

func movePageHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		var req movePageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		page, err := st.Move(r.Context(), id, req.ParentID)
		if err != nil {
			if errors.Is(err, store.ErrCycle) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			handleStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, page)
	}
}

func deletePageHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		if err := st.Delete(r.Context(), id); err != nil {
			handleStoreErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func listRevisionsHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		revs, err := st.ListRevisions(r.Context(), id)
		if err != nil {
			handleStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, revs)
	}
}

func revertHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		revisionID := chi.URLParam(r, "revisionID")

		page, err := st.Revert(r.Context(), id, revisionID)
		if err != nil {
			handleStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, page)
	}
}

func searchHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			writeJSON(w, http.StatusOK, []store.SearchResult{})
			return
		}

		results, err := st.Search(r.Context(), q)
		if err != nil {
			handleStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, results)
	}
}
