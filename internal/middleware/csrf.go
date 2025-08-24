package middleware

import (
	"net/http"
	"strings"
)

// CSRFMiddleware ajoute une protection CSRF basique
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Appliquer CSRF seulement aux requêtes modifiantes
		if isModifyingRequest(r) {
			// Vérifier l'en-tête Referer pour une protection basique
			referer := r.Header.Get("Referer")
			host := r.Header.Get("Host")

			// Pour les requêtes AJAX/API, vérifier l'origine
			if isAPIRequest(r) {
				origin := r.Header.Get("Origin")
				if origin != "" && !strings.Contains(origin, host) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					w.Write([]byte(`{"error": "CSRF protection: Invalid origin"}`))
					return
				}
			}

			// Pour les formulaires, vérifier le referer
			if referer != "" && !strings.Contains(referer, host) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error": "CSRF protection: Invalid referer"}`))
				return
			}
		}

		// Ajouter des en-têtes de sécurité
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		next.ServeHTTP(w, r)
	})
}

// isModifyingRequest vérifie si c'est une requête qui modifie des données
func isModifyingRequest(r *http.Request) bool {
	return r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" || r.Method == "PATCH"
}
