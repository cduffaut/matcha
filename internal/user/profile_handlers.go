package user

import (
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cduffaut/matcha/internal/chat"
	"github.com/cduffaut/matcha/internal/models"
	"github.com/cduffaut/matcha/internal/notifications"
	"github.com/cduffaut/matcha/internal/security"
	"github.com/cduffaut/matcha/internal/session"
	"github.com/cduffaut/matcha/internal/validation"
	"goji.io/pat"
)

// GeolocationResponse représente une réponse de géolocalisation
type GeolocationResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	City      string  `json:"city"`
	Country   string  `json:"country"`
	Method    string  `json:"method"`
	Accuracy  int     `json:"accuracy"`
}

// ProfileHandlers gère les requêtes HTTP pour les profils utilisateurs
type ProfileHandlers struct {
	profileService      *ProfileService
	notificationService notifications.NotificationService
	hub                 *chat.Hub
}

// NewProfileHandlers crée de nouveaux gestionnaires pour les profils
func NewProfileHandlers(profileService *ProfileService, notificationService notifications.NotificationService, hub *chat.Hub) *ProfileHandlers {
	return &ProfileHandlers{
		profileService:      profileService,
		notificationService: notificationService,
		hub:                 hub,
	}
}

// ProfileUpdateRequest représente les données pour la mise à jour d'un profil
type ProfileUpdateRequest struct {
	Gender           string     `json:"gender"`
	SexualPreference string     `json:"sexual_preference"`
	Biography        string     `json:"biography"`
	BirthDate        *time.Time `json:"birth_date"`
	Latitude         float64    `json:"latitude"`
	Longitude        float64    `json:"longitude"`
	LocationName     string     `json:"location_name"`
	Tags             []string   `json:"tags"`
}

type WebSocketMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// Client WebSocket
type Client struct {
	UserID int
	Conn   interface{} // WebSocket connection
	Send   chan []byte
}

// Hub gère les connexions WebSocket
type Hub struct {
	Clients    map[int]*Client
	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan []byte
}

// GetProfileHandler récupère le profil de l'utilisateur connecté
func (h *ProfileHandlers) GetProfileHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer le profil
	profile, err := h.profileService.GetProfile(session.UserID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération du profil", http.StatusInternalServerError)
		return
	}

	// Répondre avec le profil
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}

// UpdateProfileHandler met à jour le profil
func (h *ProfileHandlers) UpdateProfileHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Authentication required"}`))
		return
	}

	// Limiter la taille du body AVANT décodage
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB max

	// Décoder le corps de la requête
	var req ProfileUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "Invalid request format or too large"}`))
		return
	}

	// Nettoyer et valider les entrées
	req.Gender = validation.SanitizeInput(req.Gender)
	req.SexualPreference = validation.SanitizeInput(req.SexualPreference)
	req.Biography = validation.SanitizeInput(req.Biography)
	req.LocationName = validation.SanitizeInput(req.LocationName)

	// Valider les tailles AVANT tout traitement
	var validationErrors validation.ValidationErrors

	// Validation du genre
	if err := validation.ValidateGender(req.Gender); err != nil {
		validationErrors = append(validationErrors, err.(validation.ValidationError))
	}

	// Validation de la préférence sexuelle
	if err := validation.ValidateSexualPreference(req.SexualPreference); err != nil {
		validationErrors = append(validationErrors, err.(validation.ValidationError))
	}

	// Valider EXPLICITEMENT la taille de la biographie
	if len(req.Biography) > validation.MaxBiographyLength {
		validationErrors = append(validationErrors, validation.ValidationError{
			Field:   "biography",
			Message: fmt.Sprintf("la biographie doit contenir au maximum %d caractères (actuellement %d)", validation.MaxBiographyLength, len(req.Biography)),
		})
	}

	// Validation de la biographie (contenu)
	if err := validation.ValidateBiography(req.Biography); err != nil {
		validationErrors = append(validationErrors, err.(validation.ValidationError))
	}

	// Validation des coordonnées
	if req.Latitude != 0 || req.Longitude != 0 {
		if err := validation.ValidateCoordinates(req.Latitude, req.Longitude); err != nil {
			validationErrors = append(validationErrors, err.(validation.ValidationError))
		}
	}

	// Validation des tags si présents
	for _, tagName := range req.Tags {
		cleanTag := validation.SanitizeInput(tagName)
		if err := validation.ValidateTag(cleanTag); err != nil {
			validationErrors = append(validationErrors, validation.ValidationError{
				Field:   "tags",
				Message: fmt.Sprintf("Tag invalide '%s': %s", tagName, err.Error()),
			})
		}
	}

	// Retourner immédiatement si erreurs de validation
	if len(validationErrors) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errors":  validationErrors,
			"message": "Données invalides",
		})
		return
	}

	// Vérifier les injections SQL
	for _, field := range []struct{ value, name string }{
		{req.Biography, "biography"},
		{req.LocationName, "location"},
	} {
		if field.value != "" {
			// Utiliser une validation spécialisée pour la biographie
			if field.name == "biography" {
				if err := security.ValidateBiographyContent(field.value); err != nil {
					security.LogSuspiciousActivity(session.UserID, field.value, "/api/profile")
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(map[string]string{
						"error": "Contenu de biographie non autorisé",
					})
					return
				}
			} else {
				// Pour les autres champs, utiliser la validation normale
				if err := security.ValidateUserInput(field.value, field.name); err != nil {
					security.LogSuspiciousActivity(session.UserID, field.value, "/api/profile")
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(map[string]string{
						"error": "Données invalides détectées",
					})
					return
				}
			}
		}
	}

	// Récupérer le profil existant
	profile, err := h.profileService.GetProfile(session.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Profile retrieval error"}`))
		return
	}

	// Mettre à jour les champs
	profile.Gender = Gender(req.Gender)
	profile.SexualPreference = SexualPreference(req.SexualPreference)
	profile.Biography = req.Biography
	profile.BirthDate = req.BirthDate
	profile.Latitude = req.Latitude
	profile.Longitude = req.Longitude
	profile.LocationName = req.LocationName

	// Mettre à jour le profil
	if err := h.profileService.UpdateProfile(session.UserID, profile); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Profile update error"}`))
		return
	}

	// Mettre à jour les tags si nécessaire
	if req.Tags != nil {
		// Supprimer les anciens tags
		for _, tag := range profile.Tags {
			h.profileService.RemoveTag(session.UserID, tag.ID)
		}

		// Ajouter les nouveaux tags (déjà validés)
		for _, tagName := range req.Tags {
			cleanTag := validation.SanitizeInput(tagName)
			h.profileService.AddTag(session.UserID, cleanTag)
		}
	}

	// Récupérer le profil mis à jour
	updatedProfile, err := h.profileService.GetProfile(session.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Updated profile retrieval error"}`))
		return
	}

	// Répondre avec le profil mis à jour
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedProfile)
}

// AddTagHandler ajoute un tag
func (h *ProfileHandlers) AddTagHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Utilisateur non connecté",
		})
		return
	}

	// Décoder le corps de la requête
	var req struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Format de requête invalide",
		})
		return
	}

	// Nettoyer et valider le tag
	req.TagName = validation.SanitizeInput(req.TagName)

	if err := validation.ValidateTag(req.TagName); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	// Ajouter le tag
	if err := h.profileService.AddTag(session.UserID, req.TagName); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	// Succès
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Tag ajouté avec succès",
	})
}

// RemoveTagHandler supprime un tag du profil de l'utilisateur connecté
func (h *ProfileHandlers) RemoveTagHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer l'ID du tag
	tagID, err := strconv.Atoi(pat.Param(r, "tagID"))
	if err != nil {
		http.Error(w, "ID de tag invalide", http.StatusBadRequest)
		return
	}

	// Supprimer le tag
	if err := h.profileService.RemoveTag(session.UserID, tagID); err != nil {
		http.Error(w, "Erreur lors de la suppression du tag", http.StatusInternalServerError)
		return
	}

	// Récupérer les tags mis à jour
	tags, err := h.profileService.GetTags(session.UserID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des tags", http.StatusInternalServerError)
		return
	}

	// Répondre avec les tags
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tags)
}

// GetTagsHandler récupère les tags de l'utilisateur connecté
func (h *ProfileHandlers) GetTagsHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer les tags
	tags, err := h.profileService.GetTags(session.UserID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des tags", http.StatusInternalServerError)
		return
	}

	// Répondre avec les tags
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tags)
}

// GetAllTagsHandler récupère tous les tags disponibles
func (h *ProfileHandlers) GetAllTagsHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer tous les tags
	tags, err := h.profileService.GetAllTags()
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des tags", http.StatusInternalServerError)
		return
	}

	// Répondre avec les tags
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tags)
}

// UploadPhotoHandler - VERSION OPTIMISÉE qui fait confiance au client
func (h *ProfileHandlers) UploadPhotoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Vérification session
	session, ok := session.FromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Utilisateur non connecté",
		})
		return
	}

	// Limitation de taille
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	// Parse form
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		// Cette erreur ne devrait jamais arriver si le client fait son job
		fmt.Printf("[WARN] Client a envoyé un fichier trop volumineux: %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Fichier trop volumineux (validation côté client échouée)",
		})
		return
	}

	// Récupération fichier
	file, header, err := r.FormFile("photo")
	if err != nil {
		// Cette erreur ne devrait jamais arriver si le client fait son job
		fmt.Printf("[WARN] Erreur récupération fichier: %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Erreur récupération fichier (validation côté client échouée)",
		})
		return
	}
	defer file.Close()

	// VALIDATION MINIMALE CÔTÉ SERVEUR (le client a déjà tout fait)
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" {
		// Ne devrait jamais arriver
		fmt.Printf("[WARN] Extension non autorisée reçue: %s\n", ext)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Type de fichier non autorisé",
		})
		return
	}

	if header.Size > 8*1024*1024 || header.Size == 0 {
		// Ne devrait jamais arriver
		fmt.Printf("[WARN] Taille fichier invalide: %d bytes\n", header.Size)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Taille de fichier invalide",
		})
		return
	}

	// Lecture fichier
	fileData := make([]byte, header.Size)
	_, err = file.Read(fileData)
	if err != nil {
		fmt.Printf("[ERROR] Lecture fichier échouée: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Erreur lecture fichier",
		})
		return
	}

	// VALIDATION SÉCURISÉE SIMPLIFIÉE
	// Puisque le client a validé les basics, on se concentre sur la sécurité
	processedData, err := security.ProcessAndValidateImage(header, fileData)
	if err != nil {
		// Log détaillé côté serveur pour debug
		fmt.Printf("[SECURITY] Validation sécurisée échouée pour user %d: %v\n", session.UserID, err)

		// Réponse propre côté client
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Image rejetée pour des raisons de sécurité",
		})
		return
	}

	// Nettoyage nom
	cleanFilename := security.SanitizeFilename(header.Filename)
	if cleanFilename == "" {
		cleanFilename = "photo" + ext
	}

	isProfile := r.FormValue("is_profile") == "true"

	// Upload
	photo, err := h.profileService.UploadPhotoSecure(session.UserID, processedData, cleanFilename, isProfile)
	if err != nil {
		fmt.Printf("[ERROR] Upload échoué pour user %d: %v\n", session.UserID, err)

		// Vérifier le type d'erreur pour retourner le bon code
		errorStr := strings.ToLower(err.Error())
		if strings.Contains(errorStr, "limite") {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Limite de 5 photos atteinte",
			})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Erreur lors de l'enregistrement",
			})
		}
		return
	}

	// Succès
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Photo uploadée avec succès",
		"photo":   photo,
	})
}

// DeletePhotoHandler supprime une photo de l'utilisateur connecté
func (h *ProfileHandlers) DeletePhotoHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Utilisateur non connecté",
		})
		return
	}

	// Récupérer l'ID de la photo
	photoIDStr := pat.Param(r, "photoID")
	photoID, err := strconv.Atoi(photoIDStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "ID de photo invalide: " + photoIDStr,
		})
		return
	}

	// Supprimer la photo
	if err := h.profileService.DeletePhoto(session.UserID, photoID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Suppression échouée: %v", err),
		})
		return
	}

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Photo supprimée avec succès",
		"photoID": photoIDStr,
	})
}

// SetProfilePhotoHandler définit une photo comme photo de profil
func (h *ProfileHandlers) SetProfilePhotoHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer l'ID de la photo
	photoID, err := strconv.Atoi(pat.Param(r, "photoID"))
	if err != nil {
		http.Error(w, "ID de photo invalide", http.StatusBadRequest)
		return
	}

	// Définir la photo comme photo de profil
	if err := h.profileService.SetProfilePhoto(session.UserID, photoID); err != nil {
		http.Error(w, fmt.Sprintf("Erreur lors de la définition de la photo de profil: %v", err), http.StatusInternalServerError)
		return
	}

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Photo de profil définie avec succès",
	})
}

// GetPhotosHandler récupère les photos de l'utilisateur connecté
func (h *ProfileHandlers) GetPhotosHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer les photos
	photos, err := h.profileService.GetPhotos(session.UserID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des photos", http.StatusInternalServerError)
		return
	}

	// Répondre avec les photos
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(photos)
}

// GetUserProfileHandler récupère le profil d'un utilisateur spécifique
func (h *ProfileHandlers) GetUserProfileHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer l'ID de l'utilisateur
	userID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		http.Error(w, "ID d'utilisateur invalide", http.StatusBadRequest)
		return
	}

	// ✅ VÉRIFIER SI C'EST UNE REQUÊTE AJAX AVANT TOUT
	acceptHeader := r.Header.Get("Accept")
	isAjaxRequest := strings.Contains(r.Header.Get("X-Requested-With"), "XMLHttpRequest") ||
		strings.Contains(acceptHeader, "application/json")

	// ✅ N'ENREGISTRER LA VISITE QUE SI CE N'EST PAS AJAX
	if !isAjaxRequest {
		// Enregistrer la visite
		h.profileService.ViewProfile(session.UserID, userID)

		// Créer notification en base
		if h.notificationService != nil {
			go func() {
				if err := h.notificationService.NotifyProfileView(userID, session.UserID); err != nil {
					fmt.Printf("Erreur notification vue: %v\n", err)
				}
			}()
		}

		// Envoyer WebSocket
		go h.sendProfileViewNotification(userID, session.UserID)
	}

	// Récupérer le profil
	profile, err := h.profileService.GetProfile(userID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération du profil", http.StatusInternalServerError)
		return
	}

	// Vérifier si l'utilisateur a liké ce profil
	liked, _ := h.profileService.CheckIfLiked(session.UserID, userID)

	// Vérifier si l'utilisateur est bloqué
	blocked, _ := h.profileService.IsUserBlocked(session.UserID, userID)

	// Vérifier s'il y a un match
	matched, _ := h.profileService.CheckIfMatched(session.UserID, userID)

	// Répondre avec le profil et les informations supplémentaires
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"profile": profile,
		"liked":   liked,
		"matched": matched,
		"blocked": blocked,
	})
}

// LikeUserHandler enregistre un "like" pour un utilisateur
func (h *ProfileHandlers) LikeUserHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer l'ID de l'utilisateur à liker
	userID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		http.Error(w, "ID d'utilisateur invalide", http.StatusBadRequest)
		return
	}

	// Liker l'utilisateur
	matched, err := h.profileService.LikeUser(session.UserID, userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	// ✅ ENVOYER NOTIFICATION WEBSOCKET
	go h.sendLikeNotification(session.UserID, userID, matched)

	// Succès
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"message": "Like envoyé avec succès",
		"matched": matched,
	}
	json.NewEncoder(w).Encode(response)
}

// UnlikeUserHandler supprime un "like" pour un utilisateur
func (h *ProfileHandlers) UnlikeUserHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer l'ID de l'utilisateur à unliker
	userID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		http.Error(w, "ID d'utilisateur invalide", http.StatusBadRequest)
		return
	}

	// Unliker l'utilisateur
	if err := h.profileService.UnlikeUser(session.UserID, userID); err != nil {
		http.Error(w, "Erreur lors du unlike", http.StatusInternalServerError)
		return
	}

	// ✅ ENVOYER NOTIFICATION WEBSOCKET
	go h.sendUnlikeNotification(session.UserID, userID)

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Utilisateur unliké avec succès",
	})
}

func (h *ProfileHandlers) sendLikeNotification(fromUserID, toUserID int, isMatch bool) {
	if h.hub == nil {
		return
	}

	// Récupérer les infos de l'utilisateur qui like
	fromUser, err := h.profileService.GetUserByID(fromUserID)
	if err != nil {
		return
	}

	// ✅ CORRECTION : SEULEMENT match si c'est un match, SEULEMENT like sinon
	if isMatch {
		// Notification de match pour les deux utilisateurs
		matchMessage := WebSocketMessage{
			Type: "notification",
			Data: map[string]interface{}{
				"type":          "match",
				"from_user_id":  fromUserID,
				"from_username": fromUser.Username,
				"to_user_id":    toUserID,
				"message":       fmt.Sprintf("Vous avez un nouveau match avec %s !", fromUser.Username), // ✅ SUPPRIMER 🎉
			},
			Timestamp: time.Now(),
		}
		h.sendWebSocketToUser(toUserID, matchMessage)

		// Envoyer aussi au user qui a liké
		toUser, err := h.profileService.GetUserByID(toUserID)
		if err == nil {
			matchMessageForSender := WebSocketMessage{
				Type: "notification",
				Data: map[string]interface{}{
					"type":          "match",
					"from_user_id":  toUserID,
					"from_username": toUser.Username,
					"to_user_id":    fromUserID,
					"message":       fmt.Sprintf("Vous avez un nouveau match avec %s !", toUser.Username), // ✅ SUPPRIMER 🎉
				},
				Timestamp: time.Now(),
			}
			h.sendWebSocketToUser(fromUserID, matchMessageForSender)
		}
	} else {
		// ✅ SEULEMENT si ce n'est PAS un match
		likeMessage := WebSocketMessage{
			Type: "notification",
			Data: map[string]interface{}{
				"type":          "like",
				"from_user_id":  fromUserID,
				"from_username": fromUser.Username,
				"to_user_id":    toUserID,
				"message":       fmt.Sprintf("%s a liké votre profil !", fromUser.Username),
			},
			Timestamp: time.Now(),
		}
		h.sendWebSocketToUser(toUserID, likeMessage)
	}
}

func (h *ProfileHandlers) sendUnlikeNotification(fromUserID, toUserID int) {
	if h.hub == nil {
		return
	}

	// Récupérer les infos de l'utilisateur qui unlike
	fromUser, err := h.profileService.GetUserByID(fromUserID)
	if err != nil {
		return
	}

	// ✅ ENVOYER AVEC LE BON TYPE "notification" + sous-type
	wsMessage := WebSocketMessage{
		Type: "notification",
		Data: map[string]interface{}{
			"type":          "unlike",
			"from_user_id":  fromUserID,
			"from_username": fromUser.Username,
			"to_user_id":    toUserID,
			"message":       fmt.Sprintf("%s ne vous like plus", fromUser.Username),
		},
		Timestamp: time.Now(),
	}

	h.sendWebSocketToUser(toUserID, wsMessage)
}

func (h *ProfileHandlers) sendWebSocketToUser(userID int, message WebSocketMessage) {
	// Convertir en JSON
	data, err := json.Marshal(message)
	if err != nil {
		fmt.Printf("Erreur lors de la sérialisation du message WebSocket: %v\n", err)
		return
	}

	// Envoyer au client WebSocket
	if client, ok := h.hub.Clients[userID]; ok {
		select {
		case client.Send <- data:
			fmt.Printf("Notification WebSocket envoyée à l'utilisateur %d\n", userID)
		default:
			// Canal plein, déconnecter le client
			fmt.Printf("Canal WebSocket plein pour l'utilisateur %d, déconnexion\n", userID)
			close(client.Send)
			delete(h.hub.Clients, userID)
		}
	} else {
		fmt.Printf("Client WebSocket non trouvé pour l'utilisateur %d\n", userID)
	}
}

// ✅ AJOUTER aussi pour les vues de profil
func (h *ProfileHandlers) sendProfileViewNotification(viewedUserID, viewerID int) {
	if h.hub == nil {
		return
	}

	// Récupérer les infos de l'utilisateur qui regarde
	viewer, err := h.profileService.GetUserByID(viewerID)
	if err != nil {
		return
	}

	// Préparer le message WebSocket
	wsMessage := WebSocketMessage{
		Type: "notification",
		Data: map[string]interface{}{
			"type":          "profile_view",
			"from_user_id":  viewerID,
			"from_username": viewer.Username,
			"to_user_id":    viewedUserID,
			"message":       fmt.Sprintf("%s a consulté votre profil", viewer.Username),
		},
		Timestamp: time.Now(),
	}

	h.sendWebSocketToUser(viewedUserID, wsMessage)
}

// BlockUserHandler bloque un utilisateur
func (h *ProfileHandlers) BlockUserHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer l'ID de l'utilisateur à bloquer
	userID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		http.Error(w, "ID d'utilisateur invalide", http.StatusBadRequest)
		return
	}

	// Bloquer l'utilisateur
	if err := h.profileService.BlockUser(session.UserID, userID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Utilisateur bloqué avec succès",
	})
}

// UnblockUserHandler débloque un utilisateur
func (h *ProfileHandlers) UnblockUserHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer l'ID de l'utilisateur à débloquer
	userID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		http.Error(w, "ID d'utilisateur invalide", http.StatusBadRequest)
		return
	}

	// Débloquer l'utilisateur
	if err := h.profileService.UnblockUser(session.UserID, userID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Utilisateur débloqué avec succès",
	})
}

// GetBlockedUsersHandler récupère la liste des utilisateurs bloqués
func (h *ProfileHandlers) GetBlockedUsersHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer les utilisateurs bloqués
	blockedUsers, err := h.profileService.GetBlockedUsers(session.UserID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des utilisateurs bloqués", http.StatusInternalServerError)
		return
	}

	// Répondre avec la liste
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(blockedUsers)
}

// GetUserStatusHandler récupère le statut en ligne d'un utilisateur
func (h *ProfileHandlers) GetUserStatusHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	_, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer l'ID de l'utilisateur
	userID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		http.Error(w, "ID d'utilisateur invalide", http.StatusBadRequest)
		return
	}

	// Récupérer le statut en ligne
	isOnline, lastConnection, err := h.profileService.GetUserOnlineStatus(userID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération du statut", http.StatusInternalServerError)
		return
	}

	// Formater la réponse
	response := map[string]interface{}{
		"is_online": isOnline,
	}

	if lastConnection != nil {
		response["last_connection"] = lastConnection.Format(time.RFC3339)
		response["last_connection_formatted"] = formatRelativeTime(*lastConnection)
	}

	// Répondre avec le statut
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ReportUserHandler traite un signalement d'utilisateur
func (h *ProfileHandlers) ReportUserHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer l'ID de l'utilisateur à signaler
	userID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		http.Error(w, "ID d'utilisateur invalide", http.StatusBadRequest)
		return
	}

	// Décoder le corps de la requête
	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Format de requête invalide", http.StatusBadRequest)
		return
	}

	// Valider la raison
	if strings.TrimSpace(req.Reason) == "" {
		http.Error(w, "Raison du signalement requise", http.StatusBadRequest)
		return
	}

	// Enregistrer le signalement
	if err := h.profileService.ReportUser(session.UserID, userID, req.Reason); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Signalement enregistré avec succès",
	})
}

// ProfilePageHandler affiche la page de profil de l'utilisateur connecté
func (h *ProfileHandlers) ProfilePageHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer le profil
	profile, err := h.profileService.GetProfile(session.UserID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération du profil", http.StatusInternalServerError)
		return
	}

	// ✅ NOUVEAU : Récupérer les informations utilisateur
	user, err := h.profileService.GetUserByID(session.UserID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des informations utilisateur", http.StatusInternalServerError)
		return
	}

	// Afficher la page de profil
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Générer un timestamp pour forcer le rechargement
	cssVersion := fmt.Sprintf("%d", time.Now().Unix())

	// ✅ SÉCURITÉ : Échapper toutes les données utilisateur
	escapedFirstName := html.EscapeString(user.FirstName)
	escapedLastName := html.EscapeString(user.LastName)
	escapedEmail := html.EscapeString(user.Email)
	escapedBiography := html.EscapeString(profile.Biography)
	escapedLocation := html.EscapeString(profile.LocationName)

	// Double échappement pour les attributs HTML
	safeFirstName := strings.ReplaceAll(escapedFirstName, `"`, `&quot;`)
	safeLastName := strings.ReplaceAll(escapedLastName, `"`, `&quot;`)
	safeEmail := strings.ReplaceAll(escapedEmail, `"`, `&quot;`)

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="fr">
<head>
    <title>Profil - Matcha</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="stylesheet" href="/static/css/profile.css?v=%s">
    <link rel="stylesheet" href="/static/css/notifications_style_fix.css?v=%s">
</head>
<body>
    <header>
        <h1>Matcha</h1>
        <nav>
            <a href="/profile" class="active">Mon profil</a>
            <a href="/browse">Explorer</a>
            <a href="/notifications">
                Notifications
                <span id="notification-count"></span>
            </a>
            <a href="/chat">
                Messages
                <span id="message-count"></span>
            </a>
            <a href="/logout">Déconnexion</a>
        </nav>
    </header>
    
    <div class="container">
        <h2>Mon profil</h2>
        
        <!-- ✅ NOUVELLE SECTION : Informations personnelles -->
        <div id="user-info-form">
            <h3>Informations personnelles</h3>
            <div class="form-group">
                <label for="first_name">Prénom *</label>
                <input type="text" id="first_name" name="first_name" value="%s" required autocomplete="given-name">
            </div>
            
            <div class="form-group">
                <label for="last_name">Nom *</label>
                <input type="text" id="last_name" name="last_name" value="%s" required autocomplete="family-name">
            </div>
            
            <div class="form-group">
                <label for="email">Email *</label>
                <input type="email" id="email" name="email" value="%s" required autocomplete="email">
            </div>
            
            <button type="button" id="update-user-info">Mettre à jour mes informations</button>
        </div>
        
        <div id="profile-form">
            <h3>Informations du profil</h3>
            <div class="form-group">
                <label for="gender">Genre *</label>
                <select id="gender" name="gender" required>
                    <option value="" %s>Sélectionnez votre genre</option>
                    <option value="male" %s>Homme</option>
                    <option value="female" %s>Femme</option>
                </select>
            </div>

            <div class="form-group">
                <label for="sexual_preference">Préférence sexuelle *</label>
                <select id="sexual_preference" name="sexual_preference" required>
                    <option value="" %s>Sélectionnez votre préférence</option>
                    <option value="heterosexual" %s>Hétérosexuel(le)</option>
                    <option value="homosexual" %s>Homosexuel(le)</option>
                    <option value="bisexual" %s>Bisexuel(le)</option>
                </select>
            </div>

            <div class="form-group">
                <label for="biography">Biographie *</label>
                <textarea id="biography" name="biography" placeholder="Parlez de vous..." required>%s</textarea>
            </div>

            <div class="form-group">
                <label for="birth_date">Date de naissance *</label>
                <input type="date" id="birth_date" name="birth_date" value="%s" required>
            </div>

            <div class="form-group">
                <label for="location">Localisation</label>
                <input type="text" id="location" name="location" value="%s" readonly>
                <button type="button" id="update-location">Mettre à jour ma localisation</button>
                <button type="button" onclick="enableManualLocation()">Saisie manuelle</button>
            </div>
            
            <div class="form-group">
                <label>Tags/Intérêts *</label>
                <div id="tags-container">%s</div>
                <div id="add-tag">
                    <input type="text" id="new-tag" placeholder="Ajouter un tag (ex: #sport)">
                    <button type="button" id="add-tag-btn">Ajouter</button>
                </div>
            </div>
            
            <button type="button" id="save-profile">Sauvegarder le profil</button>
        </div>
        
        <div id="photos-section">
            <h3>Mes photos</h3>
            <div id="photos-container">%s</div>
            <div id="upload-photo">
                <h4>Ajouter une photo</h4>
                <form id="photo-form" enctype="multipart/form-data">
                    <input type="file" id="photo-file" name="photo" accept="image/*" required>
                    <label>
                        <input type="checkbox" id="is-profile" name="is_profile" value="true">
                        Photo de profil
                    </label>
                    <button type="submit">Télécharger</button>
                </form>
            </div>
        </div>
        
        <div id="stats-section">
            <h3>Statistiques</h3>
            <p>Fame rating: %d</p>
            <p><a href="/profile/visitors">Voir qui a visité mon profil</a></p>
            <p><a href="/profile/likes">Voir qui m'a liké</a></p>
            <p><a href="/profile/blocked">Gérer les utilisateurs bloqués</a></p>
        </div>
    </div>
    
	<script src="/static/js/global-error-handler.js"></script>
    <script src="/static/js/profile.js?v=%s"></script>
    <script src="/static/js/user_info.js?v=%s"></script>
    <script src="/static/js/notifications_unified.js?v=%s"></script>
    <script src="/static/js/user_status.js"></script>
    <script src="/static/js/navigation_active.js"></script>
	
</body>
</html>`,
		cssVersion, cssVersion, // Pour les CSS
		// ✅ VARIABLES SÉCURISÉES : Informations utilisateur
		safeFirstName, safeLastName, safeEmail,
		// Variables du profil
		getSelected(profile.Gender == ""),                       // Genre vide
		getSelected(profile.Gender == "male"),                   // Genre masculin
		getSelected(profile.Gender == "female"),                 // Genre féminin
		getSelected(profile.SexualPreference == ""),             // Préférence vide
		getSelected(profile.SexualPreference == "heterosexual"), // Hétérosexuel
		getSelected(profile.SexualPreference == "homosexual"),   // Homosexuel
		getSelected(profile.SexualPreference == "bisexual"),     // Bisexuel
		escapedBiography,                   // Biographie échappée
		formatBirthDate(profile.BirthDate), // Date de naissance
		escapedLocation,                    // Localisation échappée
		renderTags(profile.Tags),           // Tags
		renderPhotos(profile.Photos),       // Photos
		profile.FameRating,                 // Fame rating
		cssVersion, cssVersion, cssVersion) // Pour les JS
}

// ViewUserProfilePageHandler affiche la page de profil d'un autre utilisateur
func (h *ProfileHandlers) ViewUserProfilePageHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	userSession, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// Récupérer l'ID de l'utilisateur à voir
	userIDStr := pat.Param(r, "userID")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "ID d'utilisateur invalide", http.StatusBadRequest)
		return
	}

	// Vérifier qu'on ne regarde pas son propre profil
	if userID == userSession.UserID {
		http.Redirect(w, r, "/profile", http.StatusFound)
		return
	}

	// Vérifier si l'utilisateur est bloqué AVANT tout traitement
	blocked, err := h.profileService.IsUserBlocked(userSession.UserID, userID)
	if err != nil {
		http.Error(w, "Erreur lors de la vérification du blocage", http.StatusInternalServerError)
		return
	}

	if blocked {
		http.Error(w, "Cet utilisateur n'est pas accessible", http.StatusForbidden)
		return
	}

	// ✅ ENREGISTRER LA VISITE
	if err := h.profileService.ViewProfile(userSession.UserID, userID); err != nil {
		fmt.Printf("Erreur lors de l'enregistrement de la visite: %v\n", err)
	}

	// ✅ CRÉER LA NOTIFICATION EN BASE DE DONNÉES
	if h.notificationService != nil {
		go func() {
			if err := h.notificationService.NotifyProfileView(userID, userSession.UserID); err != nil {
				fmt.Printf("Erreur lors de la création de la notification de vue: %v\n", err)
			}
		}()
	}

	// ✅ ENVOYER LA NOTIFICATION WEBSOCKET EN TEMPS RÉEL
	go h.sendProfileViewNotification(userID, userSession.UserID)

	// Récupérer le profil de l'utilisateur
	profile, err := h.profileService.GetProfile(userID)
	if err != nil {
		http.Error(w, "Profil non trouvé", http.StatusNotFound)
		return
	}

	// Récupérer les informations de base de l'utilisateur
	user, err := h.profileService.GetUserByID(userID)
	if err != nil {
		http.Error(w, "Utilisateur non trouvé", http.StatusNotFound)
		return
	}

	// Vérifier si c'est une requête HTML (navigation) ou AJAX (action)
	acceptHeader := r.Header.Get("Accept")
	isAjaxRequest := strings.Contains(r.Header.Get("X-Requested-With"), "XMLHttpRequest") ||
		strings.Contains(acceptHeader, "application/json")

	// Vérifier si l'utilisateur connecté a liké ce profil
	liked, _ := h.profileService.CheckIfLiked(userSession.UserID, userID)

	// Vérifier s'il y a un match
	matched, _ := h.profileService.CheckIfMatched(userSession.UserID, userID)

	// Si c'est une requête AJAX, retourner JSON
	if isAjaxRequest {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"profile": profile,
			"liked":   liked,
			"matched": matched,
			"blocked": blocked,
		})
		return
	}

	// Calculer l'âge
	age := "Non spécifié"
	if profile.BirthDate != nil {
		now := time.Now()
		ageYears := now.Year() - profile.BirthDate.Year()
		if now.YearDay() < profile.BirthDate.YearDay() {
			ageYears--
		}
		age = fmt.Sprintf("%d ans", ageYears)
	}

	// ✅ SÉCURITÉ : Échapper toutes les données utilisateur
	escapedFirstName := html.EscapeString(user.FirstName)
	escapedLastName := html.EscapeString(user.LastName)
	escapedUsername := html.EscapeString(user.Username)
	escapedBiography := html.EscapeString(profile.Biography)
	escapedLocation := html.EscapeString(profile.LocationName)

	// Générer le nom à afficher (avec données échappées)
	displayName := fmt.Sprintf("%s %s (@%s)", escapedFirstName, escapedLastName, escapedUsername)

	// Générer le timestamp pour les ressources
	cssVersion := fmt.Sprintf("%d", time.Now().Unix())

	// Afficher la page
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="fr">
<head>
    <title>Profil de %s - Matcha</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="stylesheet" href="/static/css/profile.css?v=%s">
    <link rel="stylesheet" href="/static/css/notifications_style_fix.css?v=%s">
</head>
<body>
    <header>
        <h1>Matcha</h1>
        <nav>
            <a href="/profile">Mon profil</a>
            <a href="/browse">Explorer</a>
            <a href="/notifications">
                Notifications
                <span id="notification-count"></span>
            </a>
            <a href="/chat">
                Messages
                <span id="message-count"></span>
            </a>
            <a href="/logout">Déconnexion</a>
        </nav>
    </header>
    
    <div class="container">
        <div class="profile-header">
            <h2>%s</h2>
            <div class="profile-status">
                <span class="online-status" id="user-status-%d">
                    <!-- Le statut sera chargé par JavaScript -->
                </span>
            </div>
        </div>
        
        <div class="user-profile">
            <div class="profile-info">
                <p><strong>Âge:</strong> %s</p>
                <p><strong>Genre:</strong> %s</p>
                <p><strong>Orientation:</strong> %s</p>
                <p><strong>Localisation:</strong> %s</p>
                <p><strong>Fame Rating:</strong> %d/100</p>
            </div>
            
            <div class="profile-biography">
                <h3>Biographie</h3>
                <p>%s</p>
            </div>
            
            <div class="profile-interests">
                <h3>Intérêts</h3>
                %s
            </div>
            
            <div class="profile-photos">
                <h3>Photos</h3>
                %s
            </div>
            
            <div class="profile-actions">
                %s
                %s
                <button onclick="blockUser(%d)" class="block-button" type="button">🚫 Bloquer</button>
                <button onclick="reportUser(%d)" class="report-button" type="button">🚨 Signaler</button>
            </div>
        </div>
        
        <p><a href="/browse">← Retour à la recherche</a></p>
    </div>

	<script src="/static/js/global-error-handler.js"></script>
    <script src="/static/js/user_status.js?v=%s"></script>
    <script src="/static/js/navigation_active.js?v=%s"></script>
    <script src="/static/js/notifications_unified.js?v=%s"></script>
    
    <!-- Scripts spécifiques pour les interactions utilisateur -->
    <script src="/static/js/user_profile.js?v=%s"></script>
    <script src="/static/js/profile.js?v=%s"></script>

</body>
</html>`,
		displayName,            // Title (déjà échappé)
		cssVersion, cssVersion, // CSS versions
		displayName,                              // H2 (déjà échappé)
		profile.UserID,                           // Status ID
		age,                                      // Âge
		string(profile.Gender),                   // Genre
		string(profile.SexualPreference),         // Orientation
		escapedLocation,                          // ✅ Localisation échappée
		profile.FameRating,                       // Fame rating
		escapedBiography,                         // ✅ Biographie échappée
		renderUserTags(profile.Tags),             // Tags (déjà corrigé)
		renderUserPhotos(profile.Photos),         // Photos
		renderLikeButton(userID, liked, matched), // Bouton like
		renderChatButton(userID, matched),        // Bouton chat
		userID, userID,                           // IDs pour bloquer/signaler
		cssVersion, cssVersion, cssVersion, cssVersion, cssVersion) // Versions JS

	w.Write([]byte(html))
}

// LikesPageHandler affiche la page des likes
func (h *ProfileHandlers) LikesPageHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// Récupérer les likes
	likes, err := h.profileService.GetLikes(session.UserID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des likes", http.StatusInternalServerError)
		return
	}

	// Générer la page HTML
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Likes - Matcha</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="stylesheet" href="/static/css/profile.css">
    <link rel="stylesheet" href="/static/css/notifications_style_fix.css">
</head>
<body>
    <header>
        <h1>Matcha</h1>
        <nav>
            <a href="/profile">Mon profil</a>
            <a href="/browse">Explorer</a>
            <a href="/notifications">
                Notifications
                <span id="notification-count"></span>
            </a>
            <a href="/chat">
                Messages
                <span id="message-count"></span>
            </a>
            <a href="/logout">Déconnexion</a>
        </nav>
    </header>
    
    <div class="container">
        <h2>Qui m'a liké</h2>`

	if len(likes) == 0 {
		html += `<p>Aucun like pour le moment.</p>`
	} else {
		html += `<div class="likes-list">`
		for _, like := range likes {
			if user, ok := like.Liker.(*models.User); ok {
				html += fmt.Sprintf(`
                    <div class="like-item">
                        <h4>%s %s (@%s)</h4>
                        <p>Liké le: %s</p>
                        <a href="/profile/%d">Voir le profil</a>
                    </div>
                `, user.FirstName, user.LastName, user.Username,
					like.CreatedAt.Format("02/01/2006 15:04"), user.ID)
			}
		}
		html += `</div>`
	}

	html += `
        <p><a href="/profile">← Retour au profil</a></p>
    </div>

	<script src="/static/js/global-error-handler.js"></script>
	<script src="/static/js/user_status.js"></script>
    <script src="/static/js/navigation_active.js"></script>
    <script src="/static/js/notifications_unified.js"></script>
    <script src="/static/js/profile.js"></script>
</body>
</html>`

	w.Write([]byte(html))
}

// VisitorsPageHandler affiche la page des visiteurs
func (h *ProfileHandlers) VisitorsPageHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// Récupérer les visiteurs
	visitors, err := h.profileService.GetVisitors(session.UserID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des visiteurs", http.StatusInternalServerError)
		return
	}

	// Générer la page HTML
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Visiteurs - Matcha</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="stylesheet" href="/static/css/profile.css">
    <link rel="stylesheet" href="/static/css/notifications_style_fix.css">
</head>
<body>
    <header>
        <h1>Matcha</h1>
        <nav>
            <a href="/profile">Mon profil</a>
            <a href="/browse">Explorer</a>
            <a href="/notifications">
                Notifications
                <span id="notification-count"></span>
            </a>
            <a href="/chat">
                Messages  
                <span id="message-count"></span>
            </a>
            <a href="/logout">Déconnexion</a>
        </nav>
    </header>
    
    <div class="container">
        <h2>Qui a visité mon profil</h2>`

	if len(visitors) == 0 {
		html += `<p>Aucune visite pour le moment.</p>`
	} else {
		html += `<div class="visitors-list">`
		for _, visit := range visitors {
			if user, ok := visit.Visitor.(*models.User); ok {
				html += fmt.Sprintf(`
                    <div class="visitor-item">
                        <h4>%s %s (@%s)</h4>
                        <p>Visité le: %s</p>
                        <a href="/profile/%d">Voir le profil</a>
                    </div>
                `, user.FirstName, user.LastName, user.Username,
					visit.VisitedAt.Format("02/01/2006 15:04"), user.ID)
			}
		}
		html += `</div>`
	}

	html += `
        <p><a href="/profile">← Retour au profil</a></p>
    </div>

	<script src="/static/js/global-error-handler.js"></script>
    <script src="/static/js/user_status.js"></script>
    <script src="/static/js/navigation_active.js"></script>
    <script src="/static/js/notifications_unified.js"></script>
    <script src="/static/js/profile.js"></script>
</body>
</html>`

	w.Write([]byte(html))
}

// BlockedUsersPageHandler affiche la page des utilisateurs bloqués
func (h *ProfileHandlers) BlockedUsersPageHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// Récupérer les utilisateurs bloqués
	blockedUsers, err := h.profileService.GetBlockedUsers(session.UserID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des utilisateurs bloqués", http.StatusInternalServerError)
		return
	}

	// Générer la page HTML
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Utilisateurs bloqués - Matcha</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="stylesheet" href="/static/css/profile.css">
    <link rel="stylesheet" href="/static/css/notifications_style_fix.css">
</head>
<body>
    <header>
        <h1>Matcha</h1>
        <nav>
            <a href="/profile">Mon profil</a>
            <a href="/browse">Explorer</a>
            <a href="/notifications">
                Notifications
                <span id="notification-count"></span>
            </a>
            <a href="/chat">
                Messages
                <span id="message-count"></span>
            </a>
            <a href="/logout">Déconnexion</a>
        </nav>
    </header>
    
    <div class="container">
        <h2>Utilisateurs bloqués</h2>`

	if len(blockedUsers) == 0 {
		html += `<p>Aucun utilisateur bloqué.</p>`
	} else {
		html += `<div class="blocked-list">`
		for _, blocked := range blockedUsers {
			if user, ok := blocked.User.(*models.User); ok {
				html += fmt.Sprintf(`
                    <div class="blocked-item">
                        <h4>%s %s (@%s)</h4>
                        <p>Bloqué le: %s</p>
                        <button onclick="unblockUser(%d)">Débloquer</button>
                    </div>
                `, user.FirstName, user.LastName, user.Username,
					blocked.CreatedAt.Format("02/01/2006 15:04"), user.ID)
			}
		}
		html += `</div>`
	}

	html += `
        <p><a href="/profile">← Retour au profil</a></p>
        <script>
            async function unblockUser(userId) {
                if (!confirm('Voulez-vous débloquer cet utilisateur ?')) return;
                
                try {
                    const response = await fetch('/api/profile/' + userId + '/block', {
                        method: 'DELETE'
                    });
                    if (response.ok) {
                        location.reload();
                    } else {
                        alert('Erreur lors du déblocage');
                    }
                } catch (error) {
                    alert('Erreur lors du déblocage');
                }
            }
        </script>

		<script src="/static/js/global-error-handler.js"></script>
		<script src="/static/js/user_status.js"></script>
		<script src="/static/js/navigation_active.js"></script>
		<script src="/static/js/notifications_unified.js"></script>
		<script src="/static/js/profile.js"></script>
    </div>
</body>
</html>`

	w.Write([]byte(html))
}

// Fonctions utilitaires

func getSelected(condition bool) string {
	if condition {
		return "selected"
	}
	return ""
}

func getDisabled(condition bool) string {
	if condition {
		return "disabled"
	}
	return ""
}

func formatBirthDate(birthDate *time.Time) string {
	if birthDate == nil {
		return ""
	}
	return birthDate.Format("2006-01-02")
}

func formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "Il y a moins d'une minute"
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "Il y a 1 minute"
		}
		return fmt.Sprintf("Il y a %d minutes", minutes)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "Il y a 1 heure"
		}
		return fmt.Sprintf("Il y a %d heures", hours)
	} else if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "Il y a 1 jour"
		}
		return fmt.Sprintf("Il y a %d jours", days)
	} else {
		return t.Format("02/01/2006")
	}
}

// Fonction d'aide pour rendre les tags en HTML
func renderTags(tags []Tag) string {
	if len(tags) == 0 {
		return "<p>Aucun intérêt ajouté</p>"
	}

	htmlContent := ""
	for _, tag := range tags {
		// ✅ ÉCHAPPER le nom du tag
		escapedTagName := html.EscapeString(tag.Name)
		htmlContent += fmt.Sprintf(`<span class="tag">%s <button class="remove-tag" data-id="%d">×</button></span>`, escapedTagName, tag.ID)
	}
	return htmlContent
}

// Fonction d'aide pour rendre les photos en HTML
func renderPhotos(photos []Photo) string {
	if len(photos) == 0 {
		return "<p>Aucune photo ajoutée</p>"
	}

	html := ""
	for _, photo := range photos {
		profileClass := ""
		if photo.IsProfile {
			profileClass = "profile-photo"
		}
		html += fmt.Sprintf(`
            <div class="photo-container %s" data-id="%d">
                <img src="%s" alt="Photo" style="max-width: 200px; max-height: 200px;">
                <div class="photo-actions">
                    <button class="set-profile-photo" %s>Définir comme photo de profil</button>
                    <button class="delete-photo">Supprimer</button>
                </div>
            </div>
        `, profileClass, photo.ID, photo.FilePath, getDisabled(photo.IsProfile))
	}
	return html
}

// renderUserPhotos génère le HTML pour afficher les photos d'un utilisateur
func renderUserPhotos(photos []Photo) string {
	if len(photos) == 0 {
		return `<div class="no-photos">
			<img src="/static/images/default-profile.jpg" alt="Pas de photo" style="max-width: 300px; border-radius: 8px;">
		</div>`
	}

	html := `<div class="user-photos">`
	for _, photo := range photos {
		profileClass := ""
		if photo.IsProfile {
			profileClass = "profile-photo"
		}
		html += fmt.Sprintf(`
			<div class="user-photo %s">
				<img src="%s" alt="Photo" style="max-width: 200px; max-height: 200px; margin: 5px; border-radius: 8px;">
			</div>`, profileClass, photo.FilePath)
	}
	html += `</div>`
	return html
}

// renderUserTags génère le HTML pour afficher les tags d'un utilisateur
func renderUserTags(tags []Tag) string {
	if len(tags) == 0 {
		return "<p>Aucun intérêt spécifié</p>"
	}

	htmlContent := `<div class="user-tags">`
	for _, tag := range tags {
		// ✅ ÉCHAPPER le nom du tag
		escapedTagName := html.EscapeString(tag.Name)
		htmlContent += fmt.Sprintf(`<span class="tag">%s</span>`, escapedTagName)
	}
	htmlContent += `</div>`
	return htmlContent
}

// renderLikeButton génère le bouton de like/unlike
func renderLikeButton(userID int, liked bool, matched bool) string {
	if liked {
		return fmt.Sprintf(`<button onclick="unlikeUser(%d)" class="unlike-button" type="button">💔 Ne plus liker</button>`, userID)
	}
	return fmt.Sprintf(`<button onclick="likeUser(%d)" class="like-button" type="button">👍 Liker</button>`, userID)
}

// renderChatButton génère le bouton de chat si match
func renderChatButton(userID int, matched bool) string {
	if matched {
		return fmt.Sprintf(`<button onclick="openChat(%d)" class="chat-button" type="button">💬 Discuter</button>`, userID)
	}
	return ""
}

// Ajoutez ces deux méthodes à la fin de votre fichier profile_handlers.go

// GetUserOnlineStatusHandler récupère le statut en ligne d'un utilisateur
func (h *ProfileHandlers) GetUserOnlineStatusHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	_, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer l'ID de l'utilisateur
	userID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		http.Error(w, "ID d'utilisateur invalide", http.StatusBadRequest)
		return
	}

	// Récupérer le statut en ligne
	isOnline, lastConnection, err := h.profileService.GetUserOnlineStatus(userID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération du statut", http.StatusInternalServerError)
		return
	}

	// Formater la réponse
	response := map[string]interface{}{
		"is_online": isOnline,
	}

	if lastConnection != nil {
		response["last_connection"] = lastConnection.Format(time.RFC3339)
		response["last_connection_formatted"] = formatRelativeTime(*lastConnection)
	}

	// Répondre avec le statut
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// IPGeolocationHandler gère la géolocalisation par IP
func (h *ProfileHandlers) IPGeolocationHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	_, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer l'IP du client
	clientIP := getClientIP(r)

	// Pour le développement local, utiliser des coordonnées par défaut
	// Dans un vrai projet, vous utiliseriez un service de géolocalisation comme MaxMind GeoIP2
	response := GeolocationResponse{
		Latitude:  48.8566, // Paris par défaut
		Longitude: 2.3522,
		City:      "Paris",
		Country:   "France",
		Method:    "ip_approximation",
		Accuracy:  10000, // 10km d'approximation
	}

	// Si l'IP n'est pas locale, vous pourriez faire un appel à un service de géolocalisation
	if !isLocalIP(clientIP) {
		// Ici vous pourriez appeler un service de géolocalisation réel
		// Pour l'instant, on garde Paris par défaut
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Fonctions utilitaires pour IPGeolocationHandler

// getClientIP récupère l'IP réelle du client
func getClientIP(r *http.Request) string {
	// Vérifier les headers de proxy
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		// X-Forwarded-For peut contenir plusieurs IPs séparées par des virgules
		ips := strings.Split(ip, ",")
		return strings.TrimSpace(ips[0])
	}

	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	if ip := r.Header.Get("X-Client-IP"); ip != "" {
		return ip
	}

	// Fallback sur RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// isLocalIP vérifie si une IP est locale
func isLocalIP(ip string) bool {
	localIPs := []string{
		"127.0.0.1",
		"::1",
		"localhost",
	}

	for _, localIP := range localIPs {
		if ip == localIP {
			return true
		}
	}

	// Vérifier les plages privées
	if strings.HasPrefix(ip, "192.168.") ||
		strings.HasPrefix(ip, "10.") ||
		strings.HasPrefix(ip, "172.") {
		return true
	}

	return false
}
