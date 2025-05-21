package user

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/cduffaut/matcha/internal/session"
)

// BrowsingHandlers gère les requêtes HTTP pour la navigation
type BrowsingHandlers struct {
	browsingService *BrowsingService
}

// NewBrowsingHandlers crée de nouveaux gestionnaires pour la navigation
func NewBrowsingHandlers(browsingService *BrowsingService) *BrowsingHandlers {
	return &BrowsingHandlers{
		browsingService: browsingService,
	}
}

// GetSuggestionsHandler récupère des suggestions de profils
func (h *BrowsingHandlers) GetSuggestionsHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	userSession, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer les paramètres de pagination
	limit := 20
	offset := 0

	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		limitParsed, err := strconv.Atoi(limitStr)
		if err == nil && limitParsed > 0 {
			limit = limitParsed
		}
	}

	offsetStr := r.URL.Query().Get("offset")
	if offsetStr != "" {
		offsetParsed, err := strconv.Atoi(offsetStr)
		if err == nil && offsetParsed >= 0 {
			offset = offsetParsed
		}
	}

	// Récupérer les suggestions
	suggestions, err := h.browsingService.GetSuggestions(userSession.UserID, limit, offset)
	if err != nil {
		fmt.Printf("Error in GetSuggestions: %v\n", err)
		http.Error(w, "Erreur lors de la récupération des suggestions", http.StatusInternalServerError)
		return
	}

	// Répondre avec les suggestions
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(suggestions); err != nil {
		fmt.Printf("Error encoding JSON response: %v\n", err)
		http.Error(w, "Erreur lors de l'encodage de la réponse", http.StatusInternalServerError)
		return
	}
}

// SearchProfilesHandler recherche des profils selon des critères
func (h *BrowsingHandlers) SearchProfilesHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	userSession, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer les paramètres de recherche
	var options FilterOptions

	// Âge
	minAgeStr := r.URL.Query().Get("min_age")
	if minAgeStr != "" {
		minAge, err := strconv.Atoi(minAgeStr)
		if err == nil {
			options.MinAge = minAge
		}
	}

	maxAgeStr := r.URL.Query().Get("max_age")
	if maxAgeStr != "" {
		maxAge, err := strconv.Atoi(maxAgeStr)
		if err == nil {
			options.MaxAge = maxAge
		}
	}

	// Fame rating
	minFameStr := r.URL.Query().Get("min_fame")
	if minFameStr != "" {
		minFame, err := strconv.Atoi(minFameStr)
		if err == nil {
			options.MinFame = minFame
		}
	}

	maxFameStr := r.URL.Query().Get("max_fame")
	if maxFameStr != "" {
		maxFame, err := strconv.Atoi(maxFameStr)
		if err == nil {
			options.MaxFame = maxFame
		}
	}

	// Distance
	maxDistanceStr := r.URL.Query().Get("max_distance")
	if maxDistanceStr != "" {
		maxDistance, err := strconv.ParseFloat(maxDistanceStr, 64)
		if err == nil {
			options.MaxDistance = maxDistance
		}
	}

	// Tags
	tags := r.URL.Query().Get("tags")
	if tags != "" {
		options.Tags = []string{tags} // Pour simplifier, on prend un seul tag
	}

	// Tri
	options.SortBy = r.URL.Query().Get("sort_by")
	options.SortOrder = r.URL.Query().Get("sort_order")
	if options.SortOrder == "" {
		options.SortOrder = "desc"
	}

	// Debug
	fmt.Printf("Search options: %+v\n", options)

	// Pagination
	limit := 20
	offset := 0

	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		limitParsed, err := strconv.Atoi(limitStr)
		if err == nil && limitParsed > 0 {
			limit = limitParsed
		}
	}

	offsetStr := r.URL.Query().Get("offset")
	if offsetStr != "" {
		offsetParsed, err := strconv.Atoi(offsetStr)
		if err == nil && offsetParsed >= 0 {
			offset = offsetParsed
		}
	}

	// Rechercher les profils
	results, err := h.browsingService.SearchProfiles(userSession.UserID, options, limit, offset)
	if err != nil {
		fmt.Printf("Error in SearchProfiles: %v\n", err)
		http.Error(w, "Erreur lors de la recherche de profils", http.StatusInternalServerError)
		return
	}

	// Répondre avec les résultats
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		fmt.Printf("Error encoding JSON response: %v\n", err)
		http.Error(w, "Erreur lors de l'encodage de la réponse", http.StatusInternalServerError)
		return
	}
}

func (h *BrowsingHandlers) BrowsePageHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	_, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Générer un timestamp pour forcer le rechargement du CSS
	cssVersion := fmt.Sprintf("%d", time.Now().Unix())

	// Afficher la page de navigation
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate") // Éviter le cache HTTP
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	fmt.Fprintf(w, `
        <!DOCTYPE html>
        <html>
        <head>
            <title>Explorer - Matcha</title>
            <meta name="viewport" content="width=device-width, initial-scale=1">
            <link rel="stylesheet" href="/static/css/browse.css?v=%s">
        </head>
        <body>
            <header>
                <h1>Matcha</h1>
                <nav>
                    <a href="/profile">Mon profil</a>
                    <a href="/browse" class="active">Explorer</a>
                    <a href="/chat">Messages</a>
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
                            <label for="tags">Tag</label>
                            <input type="text" id="tags" name="tags" placeholder="Ex: #sport">
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
                    <!-- Les profils seront chargés ici via JavaScript -->
                    <div class="loading">Chargement des profils...</div>
                </div>
                
                <div class="pagination">
                    <button id="load-more">Charger plus</button>
                </div>
            </div>
            
            <script src="/static/js/browse.js?v=%s"></script>
        </body>
        </html>
    `, cssVersion, cssVersion)
}
