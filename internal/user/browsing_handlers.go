package user

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cduffaut/matcha/internal/session"
)

/* ---------- helpers ---------- */

func requireSessionID(r *http.Request) (int, bool) {
	s, ok := session.FromContext(r.Context())
	if !ok {
		return 0, false
	}
	return s.UserID, true
}

func okJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func errJSON(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func qInt(r *http.Request, key string, def, min int) int {
	if s := r.URL.Query().Get(key); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v >= min {
			return v
		}
	}
	return def
}

func qFloat(r *http.Request, key string, def float64) float64 {
	if s := r.URL.Query().Get(key); s != "" {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return v
		}
	}
	return def
}

func qCSVTags(r *http.Request, key string) []string {
	raw := strings.Split(r.URL.Query().Get(key), ",")
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		if tt := strings.TrimSpace(t); tt != "" {
			out = append(out, tt)
		}
	}
	return out
}

/* ---------- handlers ---------- */

type BrowsingHandlers struct {
	browsingService *BrowsingService
}

func NewBrowsingHandlers(b *BrowsingService) *BrowsingHandlers { return &BrowsingHandlers{browsingService: b} }

func (h *BrowsingHandlers) GetSuggestionsHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSessionID(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}

	limit := qInt(r, "limit", 20, 1)
	offset := qInt(r, "offset", 0, 0)

	suggestions, err := h.browsingService.GetSuggestions(uid, limit, offset)
	if err != nil {
		if strings.Contains(err.Error(), "votre profil doit être complété") {
			okJSON(w, map[string]any{
				"suggestions":        []any{},
				"message":            "Complétez votre profil pour voir des suggestions",
				"profile_incomplete": true,
			})
			return
		}
		errJSON(w, http.StatusInternalServerError, "Erreur lors de la récupération des suggestions")
		return
	}

	okJSON(w, map[string]any{
		"suggestions":        suggestions,
		"profile_incomplete": false,
	})
}

func (h *BrowsingHandlers) SearchProfilesHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSessionID(r)
	if !ok {
		errJSON(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}

	opt := FilterOptions{
		MinAge:      qInt(r, "min_age", 0, 0),
		MaxAge:      qInt(r, "max_age", 0, 0),
		MinFame:     qInt(r, "min_fame", 0, 0),
		MaxFame:     qInt(r, "max_fame", 0, 0),
		MaxDistance: qFloat(r, "max_distance", 0),
		Tags:        qCSVTags(r, "tags"),
		SortBy:      r.URL.Query().Get("sort_by"),
		SortOrder:   r.URL.Query().Get("sort_order"),
	}
	if opt.SortOrder == "" {
		opt.SortOrder = "desc"
	}

	limit := qInt(r, "limit", 20, 1)
	offset := qInt(r, "offset", 0, 0)

	results, err := h.browsingService.SearchProfiles(uid, opt, limit, offset)
	if err != nil {
		if strings.Contains(err.Error(), "votre profil doit être complété") {
			okJSON(w, map[string]any{
				"results":            []any{},
				"message":            "Complétez votre profil pour effectuer des recherches",
				"profile_incomplete": true,
			})
			return
		}
		errJSON(w, http.StatusInternalServerError, "Erreur lors de la recherche de profils")
		return
	}

	okJSON(w, map[string]any{
		"results":            results,
		"profile_incomplete": false,
	})
}

func (h *BrowsingHandlers) BrowsePageHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSessionID(r); !ok {
		errJSON(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}

	ver := fmt.Sprintf("%d", time.Now().Unix())
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
  <title>Explorer - Matcha</title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" href="/static/css/browse.css?v=%s">
  <link rel="stylesheet" href="/static/css/notifications_style_fix.css">
</head>
<body>
  <header>
    <h1>Matcha</h1>
    <nav>
      <a href="/profile">Mon profil</a>
      <a href="/browse">Explorer</a>
      <a href="/notifications">Notifications <span id="notification-count"></span></a>
      <a href="/chat">Messages <span id="message-count"></span></a>
      <a href="/logout">Déconnexion</a>
    </nav>
  </header>

  <div class="container">
    <h2>Suggestions pour vous</h2>

    <div class="filters">
      <h3>Filtrer</h3>
      <form id="search-form">
        <div class="form-group">
          <label for="min_age">Âge minimum</label>
          <input type="number" id="min_age" name="min_age" min="18" max="100">
        </div>
        <div class="form-group">
          <label for="max_age">Âge maximum</label>
          <input type="number" id="max_age" name="max_age" min="18" max="100">
        </div>
        <div class="form-group">
          <label for="min_fame">Fame rating minimum</label>
          <input type="number" id="min_fame" name="min_fame" min="0" max="100">
        </div>
        <div class="form-group">
          <label for="max_fame">Fame rating maximum</label>
          <input type="number" id="max_fame" name="max_fame" min="0" max="100">
        </div>
        <div class="form-group">
          <label for="max_distance">Distance maximum (km)</label>
          <input type="number" id="max_distance" name="max_distance" min="0">
        </div>
        <div class="form-group">
          <label for="tags-search">Tags d'intérêt :</label>
          <div class="tags-search-group">
            <div class="tags-input-row">
              <input type="text" id="tags-search" placeholder="Ex: sport, voyage, cuisine (# automatique)...">
              <button type="button" id="add-tag-btn">Ajouter</button>
            </div>
            <div id="selected-tags"></div>
            <div id="tag-suggestions"></div>
          </div>
        </div>
        <div class="form-group">
          <label for="sort_by">Trier par</label>
          <select id="sort_by" name="sort_by">
            <option value="compatibility">Compatibilité</option>
            <option value="age">Âge</option>
            <option value="distance">Distance</option>
            <option value="fame">Fame rating</option>
            <option value="common_tags">Tags communs</option>
          </select>
        </div>
        <div class="form-group">
          <label for="sort_order">Ordre</label>
          <select id="sort_order" name="sort_order">
            <option value="desc">Décroissant</option>
            <option value="asc">Croissant</option>
          </select>
        </div>
        <button type="submit">Rechercher</button>
      </form>
    </div>

    <div id="profiles-container">
      <div class="loading">Chargement des profils...</div>
    </div>

    <div class="pagination">
      <button id="load-more">Charger plus</button>
    </div>
  </div>

  <script src="/static/js/global-error-handler.js"></script>
  <script src="/static/js/browse.js?v=%s"></script>
  <script src="/static/js/user_status.js"></script>
  <script src="/static/js/navigation_active.js"></script>
  <script src="/static/js/notifications_unified.js"></script>
</body>
</html>`, ver, ver)
}
