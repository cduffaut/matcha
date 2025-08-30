package middleware

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// bloque les requêtes cross-origin sur méthodes mutantes.
// Origin prioritaire ; fallback sur Referer si absent.
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if isModifyingMethod(r.Method) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if !sameOrigin(origin, r.Host) {
					forbidCSRF(w, r, "Invalid origin")
					return
				}
			} else {
				ref := r.Header.Get("Referer")
				if ref != "" && !sameOrigin(ref, r.Host) {
					forbidCSRF(w, r, "Invalid referer")
					return
				}
			}
		}

		// Headers de sécurité légers et sûrs par défaut
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")

		next.ServeHTTP(w, r)
	})
}

func isModifyingMethod(m string) bool {
	switch m {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// compare l'hôte (avec port) d'une URL header à r.Host.
func sameOrigin(u, host string) bool {
	pu, err := url.Parse(u)
	if err != nil || pu.Host == "" {
		return false
	}
	// compare insensiblement à la casse ; garde le port si présent.
	return strings.EqualFold(pu.Host, host)
}

func forbidCSRF(w http.ResponseWriter, r *http.Request, msg string) {
	w.WriteHeader(http.StatusForbidden)
	ct := "text/html; charset=utf-8"
	if isAPIRequest(r) {
		ct = "application/json"
	}
	w.Header().Set("Content-Type", ct)
	if ct == "application/json" {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "CSRF protection: " + msg})
		return
	}
	_, _ = w.Write([]byte("CSRF protection: " + msg))
}
