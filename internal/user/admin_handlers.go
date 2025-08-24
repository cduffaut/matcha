package user

import (
	"encoding/json"
	"net/http"
	"strconv"

	"goji.io/pat"
)

// GetReportsHandler récupère tous les signalements (pour les admins)
func (h *ProfileHandlers) GetReportsHandler(w http.ResponseWriter, r *http.Request) {
	// Note: Dans un vrai projet, il faudrait vérifier que l'utilisateur est admin

	// Récupérer tous les signalements non traités
	reports, err := h.profileService.GetAllReports()
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des signalements", http.StatusInternalServerError)
		return
	}

	// Répondre avec les signalements
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reports)
}

// ProcessReportHandler traite un signalement (marque comme traité)
func (h *ProfileHandlers) ProcessReportHandler(w http.ResponseWriter, r *http.Request) {
	// Note: Dans un vrai projet, il faudrait vérifier que l'utilisateur est admin

	// Récupérer l'ID du signalement
	reportID, err := strconv.Atoi(pat.Param(r, "reportID"))
	if err != nil {
		http.Error(w, "ID de signalement invalide", http.StatusBadRequest)
		return
	}

	// Décoder la requête
	var req struct {
		AdminComment string `json:"admin_comment"`
		Action       string `json:"action"` // "dismiss", "warn", "suspend", "ban"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Format de requête invalide", http.StatusBadRequest)
		return
	}

	// Traiter le signalement
	if err := h.profileService.ProcessReport(reportID, req.AdminComment, req.Action); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Signalement traité avec succès",
	})
}
