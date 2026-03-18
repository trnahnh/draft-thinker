package gateway

import (
	"net/http"
	"strings"

	"github.com/trnahnh/draft-thinker/internal/cache"
)

type cacheEvictHandler struct {
	cache cache.Store
}

func newCacheEvictHandler(store cache.Store) *cacheEvictHandler {
	return &cacheEvictHandler{cache: store}
}

func (h *cacheEvictHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeBadRequest(w, "method not allowed, use DELETE")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/v1/cache/")
	if id == "" {
		writeBadRequest(w, "cache entry id is required")
		return
	}

	if err := h.cache.Evict(r.Context(), id); err != nil {
		writeInternalError(w, "cache eviction failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
