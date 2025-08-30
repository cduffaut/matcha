package user

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
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

/* --------- Types --------- */

// pour la réponse de géolocalisation.
type GeolocationResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	City      string  `json:"city"`
	Country   string  `json:"country"`
	Method    string  `json:"method"`
	Accuracy  int     `json:"accuracy"`
}

// gère les endpoints profil.
type ProfileHandlers struct {
	profileService      *ProfileService
	notificationService notifications.NotificationService
	hub                 *chat.Hub
}

// construit le handler.
func NewProfileHandlers(profileService *ProfileService, notificationService notifications.NotificationService, hub *chat.Hub) *ProfileHandlers {
	return &ProfileHandlers{profileService: profileService, notificationService: notificationService, hub: hub}
}

// payload de mise à jour de profil.
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

// format de message WS.
type WebSocketMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

/* --------- Helpers génériques --------- */

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeOK(w http.ResponseWriter, v any) {
	writeJSON(w, http.StatusOK, v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func requireSession(r *http.Request) (int, bool) {
	sess, ok := session.FromContext(r.Context())
	if !ok {
		return 0, false
	}
	return sess.UserID, true
}

func assetsVersion() string { return fmt.Sprintf("%d", time.Now().Unix()) }

/* --------- API JSON --------- */

// renvoie le profil courant.
func (h *ProfileHandlers) GetProfileHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}
	profile, err := h.profileService.GetProfile(uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Erreur lors de la récupération du profil")
		return
	}
	writeOK(w, profile)
}

// met à jour le profil.
func (h *ProfileHandlers) UpdateProfileHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req ProfileUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request format or too large")
		return
	}

	req.Gender = validation.SanitizeInput(req.Gender)
	req.SexualPreference = validation.SanitizeInput(req.SexualPreference)
	req.Biography = validation.SanitizeInput(req.Biography)
	req.LocationName = validation.SanitizeInput(req.LocationName)

	var verr validation.ValidationErrors
	if err := validation.ValidateGender(req.Gender); err != nil {
		verr = append(verr, err.(validation.ValidationError))
	}
	if err := validation.ValidateSexualPreference(req.SexualPreference); err != nil {
		verr = append(verr, err.(validation.ValidationError))
	}
	if len(req.Biography) > validation.MaxBiographyLength {
		verr = append(verr, validation.ValidationError{
			Field:   "biography",
			Message: fmt.Sprintf("la biographie doit contenir au maximum %d caractères (actuellement %d)", validation.MaxBiographyLength, len(req.Biography)),
		})
	}
	if err := validation.ValidateBiography(req.Biography); err != nil {
		verr = append(verr, err.(validation.ValidationError))
	}
	if req.Latitude != 0 || req.Longitude != 0 {
		if err := validation.ValidateCoordinates(req.Latitude, req.Longitude); err != nil {
			verr = append(verr, err.(validation.ValidationError))
		}
	}
	for _, tagName := range req.Tags {
		if err := validation.ValidateTag(validation.SanitizeInput(tagName)); err != nil {
			verr = append(verr, validation.ValidationError{Field: "tags", Message: fmt.Sprintf("Tag invalide '%s': %s", tagName, err.Error())})
		}
	}
	if len(verr) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"errors": verr, "message": "Données invalides"})
		return
	}

	for _, f := range []struct{ v, n string }{{req.Biography, "biography"}, {req.LocationName, "location"}} {
		if f.v == "" {
			continue
		}
		if f.n == "biography" {
			if err := security.ValidateBiographyContent(f.v); err != nil {
				security.LogSuspiciousActivity(uid, f.v, "/api/profile")
				writeError(w, http.StatusBadRequest, "Contenu de biographie non autorisé")
				return
			}
		} else if err := security.ValidateUserInput(f.v, f.n); err != nil {
			security.LogSuspiciousActivity(uid, f.v, "/api/profile")
			writeError(w, http.StatusBadRequest, "Données invalides détectées")
			return
		}
	}

	profile, err := h.profileService.GetProfile(uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Profile retrieval error")
		return
	}

	profile.Gender = Gender(req.Gender)
	profile.SexualPreference = SexualPreference(req.SexualPreference)
	profile.Biography = req.Biography
	profile.BirthDate = req.BirthDate
	profile.Latitude = req.Latitude
	profile.Longitude = req.Longitude
	profile.LocationName = req.LocationName

	if err := h.profileService.UpdateProfile(uid, profile); err != nil {
		writeError(w, http.StatusInternalServerError, "Profile update error")
		return
	}

	if req.Tags != nil {
		for _, tag := range profile.Tags {
			_ = h.profileService.RemoveTag(uid, tag.ID)
		}
		for _, t := range req.Tags {
			_ = h.profileService.AddTag(uid, validation.SanitizeInput(t))
		}
	}

	updated, err := h.profileService.GetProfile(uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Updated profile retrieval error")
		return
	}
	writeOK(w, updated)
}

// ajoute un tag.
func (h *ProfileHandlers) AddTagHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}
	var req struct{ TagName string `json:"tag_name"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Format de requête invalide")
		return
	}
	req.TagName = validation.SanitizeInput(req.TagName)
	if err := validation.ValidateTag(req.TagName); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.profileService.AddTag(uid, req.TagName); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, map[string]string{"message": "Tag ajouté avec succès"})
}

// supprime un tag.
func (h *ProfileHandlers) RemoveTagHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}
	tagID, err := strconv.Atoi(pat.Param(r, "tagID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "ID de tag invalide")
		return
	}
	if err := h.profileService.RemoveTag(uid, tagID); err != nil {
		writeError(w, http.StatusInternalServerError, "Erreur lors de la suppression du tag")
		return
	}
	tags, err := h.profileService.GetTags(uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Erreur lors de la récupération des tags")
		return
	}
	writeOK(w, tags)
}

// renvoie les tags de l’utilisateur.
func (h *ProfileHandlers) GetTagsHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}
	tags, err := h.profileService.GetTags(uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Erreur lors de la récupération des tags")
		return
	}
	writeOK(w, tags)
}

// renvoie tous les tags.
func (h *ProfileHandlers) GetAllTagsHandler(w http.ResponseWriter, r *http.Request) {
	tags, err := h.profileService.GetAllTags()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Erreur lors de la récupération des tags")
		return
	}
	writeOK(w, tags)
}

// reçoit et enregistre une photo.
func (h *ProfileHandlers) UploadPhotoHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "Fichier trop volumineux")
		return
	}

	file, header, err := r.FormFile("photo")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Erreur récupération fichier")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" {
		writeError(w, http.StatusBadRequest, "Type de fichier non autorisé")
		return
	}
	if header.Size == 0 || header.Size > 8*1024*1024 {
		writeError(w, http.StatusBadRequest, "Taille de fichier invalide")
		return
	}

	fileData, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Erreur lecture fichier")
		return
	}

	processedData, err := security.ProcessAndValidateImage(header, fileData)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Image rejetée pour des raisons de sécurité")
		return
	}

	cleanFilename := security.SanitizeFilename(header.Filename)
	if cleanFilename == "" {
		cleanFilename = "photo" + ext
	}

	isProfile := r.FormValue("is_profile") == "true"
	photo, err := h.profileService.UploadPhotoSecure(uid, processedData, cleanFilename, isProfile)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "limite") {
			writeError(w, http.StatusBadRequest, "Limite de 5 photos atteinte")
			return
		}
		writeError(w, http.StatusInternalServerError, "Erreur lors de l'enregistrement")
		return
	}

	writeOK(w, map[string]any{"success": true, "message": "Photo uploadée avec succès", "photo": photo})
}

// supprime une photo.
func (h *ProfileHandlers) DeletePhotoHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}
	photoIDStr := pat.Param(r, "photoID")
	photoID, err := strconv.Atoi(photoIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ID de photo invalide: "+photoIDStr)
		return
	}
	if err := h.profileService.DeletePhoto(uid, photoID); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Suppression échouée: %v", err))
		return
	}
	writeOK(w, map[string]string{"message": "Photo supprimée avec succès", "photoID": photoIDStr})
}

// définit la photo de profil.
func (h *ProfileHandlers) SetProfilePhotoHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}
	photoID, err := strconv.Atoi(pat.Param(r, "photoID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "ID de photo invalide")
		return
	}
	if err := h.profileService.SetProfilePhoto(uid, photoID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Erreur lors de la définition de la photo de profil: %v", err))
		return
	}
	writeOK(w, map[string]string{"message": "Photo de profil définie avec succès"})
}

// renvoie les photos de l’utilisateur.
func (h *ProfileHandlers) GetPhotosHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}
	photos, err := h.profileService.GetPhotos(uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Erreur lors de la récupération des photos")
		return
	}
	writeOK(w, photos)
}

// renvoie le profil d’un utilisateur (et log la visite si non-AJAX).
func (h *ProfileHandlers) GetUserProfileHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}
	userID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "ID d'utilisateur invalide")
		return
	}

	isAjax := strings.Contains(r.Header.Get("X-Requested-With"), "XMLHttpRequest") ||
		strings.Contains(r.Header.Get("Accept"), "application/json")

	if !isAjax {
		_ = h.profileService.ViewProfile(uid, userID)
		if h.notificationService != nil {
			go h.notificationService.NotifyProfileView(userID, uid)
		}
		go h.sendProfileViewNotification(userID, uid)
	}

	profile, err := h.profileService.GetProfile(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Erreur lors de la récupération du profil")
		return
	}
	liked, _ := h.profileService.CheckIfLiked(uid, userID)
	blocked, _ := h.profileService.IsUserBlocked(uid, userID)
	matched, _ := h.profileService.CheckIfMatched(uid, userID)

	writeOK(w, map[string]any{"profile": profile, "liked": liked, "matched": matched, "blocked": blocked})
}

// enregistre un like et notifie.
func (h *ProfileHandlers) LikeUserHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}
	userID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "ID d'utilisateur invalide")
		return
	}
	matched, err := h.profileService.LikeUser(uid, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	go h.sendLikeNotification(uid, userID, matched)
	writeOK(w, map[string]any{"message": "Like envoyé avec succès", "matched": matched})
}

// supprime un like et notifie.
func (h *ProfileHandlers) UnlikeUserHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}
	userID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "ID d'utilisateur invalide")
		return
	}
	if err := h.profileService.UnlikeUser(uid, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "Erreur lors du unlike")
		return
	}
	go h.sendUnlikeNotification(uid, userID)
	writeOK(w, map[string]string{"message": "Utilisateur unliké avec succès"})
}

// bloque un utilisateur.
func (h *ProfileHandlers) BlockUserHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}
	userID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "ID d'utilisateur invalide")
		return
	}
	if err := h.profileService.BlockUser(uid, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, map[string]string{"message": "Utilisateur bloqué avec succès"})
}

// débloque un utilisateur.
func (h *ProfileHandlers) UnblockUserHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}
	userID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "ID d'utilisateur invalide")
		return
	}
	if err := h.profileService.UnblockUser(uid, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, map[string]string{"message": "Utilisateur débloqué avec succès"})
}

// renvoie la liste des utilisateurs bloqués.
func (h *ProfileHandlers) GetBlockedUsersHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}
	blockedUsers, err := h.profileService.GetBlockedUsers(uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Erreur lors de la récupération des utilisateurs bloqués")
		return
	}
	writeOK(w, blockedUsers)
}

// renvoie le statut en ligne + dernière connexion.
func (h *ProfileHandlers) GetUserStatusHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(r); !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}
	userID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "ID d'utilisateur invalide")
		return
	}
	isOnline, lastConn, err := h.profileService.GetUserOnlineStatus(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Erreur lors de la récupération du statut")
		return
	}
	resp := map[string]any{"is_online": isOnline}
	if lastConn != nil {
		resp["last_connection"] = lastConn.Format(time.RFC3339)
		resp["last_connection_formatted"] = formatRelativeTime(*lastConn)
	}
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	writeOK(w, resp)
}

// enregistre un signalement.
func (h *ProfileHandlers) ReportUserHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}
	userID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "ID d'utilisateur invalide")
		return
	}
	var req struct{ Reason string `json:"reason"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Format de requête invalide")
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		writeError(w, http.StatusBadRequest, "Raison du signalement requise")
		return
	}
	if err := h.profileService.ReportUser(uid, userID, req.Reason); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, map[string]string{"message": "Signalement enregistré avec succès"})
}

/* --------- Pages HTML (sans templates) --------- */

// rend la page HTML de son propre profil.
func (h *ProfileHandlers) ProfilePageHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}
	profile, err := h.profileService.GetProfile(uid)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération du profil", http.StatusInternalServerError)
		return
	}
	user, err := h.profileService.GetUserByID(uid)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des informations utilisateur", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	ver := assetsVersion()

	escapedFirstName := html.EscapeString(user.FirstName)
	escapedLastName := html.EscapeString(user.LastName)
	escapedEmail := html.EscapeString(user.Email)
	escapedBiography := html.EscapeString(profile.Biography)
	escapedLocation := html.EscapeString(profile.LocationName)

	safeFirstName := strings.ReplaceAll(escapedFirstName, `"`, `&quot;`)
	safeLastName := strings.ReplaceAll(escapedLastName, `"`, `&quot;`)
	safeEmail := strings.ReplaceAll(escapedEmail, `"`, `&quot;`)

	page := fmt.Sprintf(`<!DOCTYPE html>
<html lang="fr">
<head>
  <title>Profil - Matcha</title>
  <meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" href="/static/css/profile.css?v=%s">
  <link rel="stylesheet" href="/static/css/notifications_style_fix.css?v=%s">
</head>
<body>
  <header>
    <h1>Matcha</h1>
    <nav>
      <a href="/profile" class="active">Mon profil</a>
      <a href="/browse">Explorer</a>
      <a href="/notifications">Notifications <span id="notification-count"></span></a>
      <a href="/chat">Messages <span id="message-count"></span></a>
      <a href="/logout">Déconnexion</a>
    </nav>
  </header>
  <div class="container">
    <h2>Mon profil</h2>

    <div id="user-info-form">
      <h3>Informations personnelles</h3>
      <div class="form-group"><label for="first_name">Prénom *</label>
        <input type="text" id="first_name" name="first_name" value="%s" required></div>
      <div class="form-group"><label for="last_name">Nom *</label>
        <input type="text" id="last_name" name="last_name" value="%s" required></div>
      <div class="form-group"><label for="email">Email *</label>
        <input type="email" id="email" name="email" value="%s" required></div>
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
        <textarea id="biography" name="biography" required>%s</textarea>
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
          <label><input type="checkbox" id="is-profile" name="is_profile" value="true"> Photo de profil</label>
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
</body></html>`,
		ver, ver,
		safeFirstName, safeLastName, safeEmail,
		getSelected(profile.Gender == ""),
		getSelected(profile.Gender == "male"),
		getSelected(profile.Gender == "female"),
		getSelected(profile.SexualPreference == ""),
		getSelected(profile.SexualPreference == "heterosexual"),
		getSelected(profile.SexualPreference == "homosexual"),
		getSelected(profile.SexualPreference == "bisexual"),
		escapedBiography,
		formatBirthDate(profile.BirthDate),
		escapedLocation,
		renderTags(profile.Tags),
		renderPhotos(profile.Photos),
		profile.FameRating,
		ver, ver, ver,
	)

	_, _ = w.Write([]byte(page))
}

// rend la page HTML d’un autre profil.
func (h *ProfileHandlers) ViewUserProfilePageHandler(w http.ResponseWriter, r *http.Request) {
	sessID, ok := requireSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	userID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		http.Error(w, "ID d'utilisateur invalide", http.StatusBadRequest)
		return
	}
	if userID == sessID {
		http.Redirect(w, r, "/profile", http.StatusFound)
		return
	}

	blocked, err := h.profileService.IsUserBlocked(sessID, userID)
	if err != nil {
		http.Error(w, "Erreur lors de la vérification du blocage", http.StatusInternalServerError)
		return
	}
	if blocked {
		http.Error(w, "Cet utilisateur n'est pas accessible", http.StatusForbidden)
		return
	}

	if err := h.profileService.ViewProfile(sessID, userID); err == nil {
		if h.notificationService != nil {
			go h.notificationService.NotifyProfileView(userID, sessID)
		}
		go h.sendProfileViewNotification(userID, sessID)
	}

	profile, err := h.profileService.GetProfile(userID)
	if err != nil {
		http.Error(w, "Profil non trouvé", http.StatusNotFound)
		return
	}
	user, err := h.profileService.GetUserByID(userID)
	if err != nil {
		http.Error(w, "Utilisateur non trouvé", http.StatusNotFound)
		return
	}

	liked, _ := h.profileService.CheckIfLiked(sessID, userID)
	matched, _ := h.profileService.CheckIfMatched(sessID, userID)

	age := "Non spécifié"
	if profile.BirthDate != nil {
		now := time.Now()
		y := now.Year() - profile.BirthDate.Year()
		if now.YearDay() < profile.BirthDate.YearDay() {
			y--
		}
		age = fmt.Sprintf("%d ans", y)
	}

	escapedFirstName := html.EscapeString(user.FirstName)
	escapedLastName := html.EscapeString(user.LastName)
	escapedUsername := html.EscapeString(user.Username)
	escapedBiography := html.EscapeString(profile.Biography)
	escapedLocation := html.EscapeString(profile.LocationName)
	displayName := fmt.Sprintf("%s %s (@%s)", escapedFirstName, escapedLastName, escapedUsername)
	ver := assetsVersion()

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	page := fmt.Sprintf(`<!DOCTYPE html>
<html lang="fr">
<head>
  <title>Profil de %s - Matcha</title>
  <meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" href="/static/css/profile.css?v=%s">
  <link rel="stylesheet" href="/static/css/notifications_style_fix.css?v=%s">
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
    <div class="profile-header">
      <h2>%s</h2>
      <div class="profile-status"><span class="online-status" id="user-status-%d"></span></div>
    </div>

    <div class="user-profile">
      <div class="profile-info">
        <p><strong>Âge:</strong> %s</p>
        <p><strong>Genre:</strong> %s</p>
        <p><strong>Orientation:</strong> %s</p>
        <p><strong>Localisation:</strong> %s</p>
        <p><strong>Fame Rating:</strong> %d/100</p>
      </div>
      <div class="profile-biography"><h3>Biographie</h3><p>%s</p></div>
      <div class="profile-interests"><h3>Intérêts</h3>%s</div>
      <div class="profile-photos"><h3>Photos</h3>%s</div>
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
  <script src="/static/js/user_profile.js?v=%s"></script>
  <script src="/static/js/profile.js?v=%s"></script>
</body></html>`,
		displayName, ver, ver,
		displayName, profile.UserID, age, string(profile.Gender), string(profile.SexualPreference), escapedLocation,
		profile.FameRating, escapedBiography, renderUserTags(profile.Tags), renderUserPhotos(profile.Photos),
		renderLikeButton(userID, liked, matched), renderChatButton(userID, matched),
		userID, userID, ver, ver, ver, ver, ver,
	)

	_, _ = w.Write([]byte(page))
}

// rend la page HTML des likes reçus.
func (h *ProfileHandlers) LikesPageHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	likes, err := h.profileService.GetLikes(uid)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des likes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	page := `<!DOCTYPE html>
<html><head>
  <title>Likes - Matcha</title>
  <meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" href="/static/css/profile.css">
  <link rel="stylesheet" href="/static/css/notifications_style_fix.css">
</head><body>
  <header><h1>Matcha</h1>
    <nav>
      <a href="/profile">Mon profil</a>
      <a href="/browse">Explorer</a>
      <a href="/notifications">Notifications <span id="notification-count"></span></a>
      <a href="/chat">Messages <span id="message-count"></span></a>
      <a href="/logout">Déconnexion</a>
    </nav>
  </header>
  <div class="container"><h2>Qui m'a liké</h2>`
	if len(likes) == 0 {
		page += `<p>Aucun like pour le moment.</p>`
	} else {
		page += `<div class="likes-list">`
		for _, like := range likes {
			if user, ok := like.Liker.(*models.User); ok {
				page += fmt.Sprintf(`
        <div class="like-item">
          <h4>%s %s (@%s)</h4>
          <p>Liké le: %s</p>
          <a href="/profile/%d">Voir le profil</a>
        </div>`, user.FirstName, user.LastName, user.Username, like.CreatedAt.Format("02/01/2006 15:04"), user.ID)
			}
		}
		page += `</div>`
	}
	page += `
    <p><a href="/profile">← Retour au profil</a></p>
  </div>
  <script src="/static/js/global-error-handler.js"></script>
  <script src="/static/js/user_status.js"></script>
  <script src="/static/js/navigation_active.js"></script>
  <script src="/static/js/notifications_unified.js"></script>
  <script src="/static/js/profile.js"></script>
</body></html>`

	_, _ = w.Write([]byte(page))
}

// rend la page HTML des visiteurs.
func (h *ProfileHandlers) VisitorsPageHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	visitors, err := h.profileService.GetVisitors(uid)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des visiteurs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	page := `<!DOCTYPE html>
<html><head>
  <title>Visiteurs - Matcha</title>
  <meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" href="/static/css/profile.css">
  <link rel="stylesheet" href="/static/css/notifications_style_fix.css">
</head><body>
  <header><h1>Matcha</h1>
    <nav>
      <a href="/profile">Mon profil</a>
      <a href="/browse">Explorer</a>
      <a href="/notifications">Notifications <span id="notification-count"></span></a>
      <a href="/chat">Messages <span id="message-count"></span></a>
      <a href="/logout">Déconnexion</a>
    </nav>
  </header>
  <div class="container"><h2>Qui a visité mon profil</h2>`
	if len(visitors) == 0 {
		page += `<p>Aucune visite pour le moment.</p>`
	} else {
		page += `<div class="visitors-list">`
		for _, visit := range visitors {
			if user, ok := visit.Visitor.(*models.User); ok {
				page += fmt.Sprintf(`
        <div class="visitor-item">
          <h4>%s %s (@%s)</h4>
          <p>Visité le: %s</p>
          <a href="/profile/%d">Voir le profil</a>
        </div>`, user.FirstName, user.LastName, user.Username, visit.VisitedAt.Format("02/01/2006 15:04"), user.ID)
			}
		}
		page += `</div>`
	}
	page += `
    <p><a href="/profile">← Retour au profil</a></p>
  </div>
  <script src="/static/js/global-error-handler.js"></script>
  <script src="/static/js/user_status.js"></script>
  <script src="/static/js/navigation_active.js"></script>
  <script src="/static/js/notifications_unified.js"></script>
  <script src="/static/js/profile.js"></script>
</body></html>`

	_, _ = w.Write([]byte(page))
}

// rend la page HTML des utilisateurs bloqués.
func (h *ProfileHandlers) BlockedUsersPageHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	blockedUsers, err := h.profileService.GetBlockedUsers(uid)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des utilisateurs bloqués", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	page := `<!DOCTYPE html>
<html><head>
  <title>Utilisateurs bloqués - Matcha</title>
  <meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" href="/static/css/profile.css">
  <link rel="stylesheet" href="/static/css/notifications_style_fix.css">
</head><body>
  <header><h1>Matcha</h1>
    <nav>
      <a href="/profile">Mon profil</a>
      <a href="/browse">Explorer</a>
      <a href="/notifications">Notifications <span id="notification-count"></span></a>
      <a href="/chat">Messages <span id="message-count"></span></a>
      <a href="/logout">Déconnexion</a>
    </nav>
  </header>
  <div class="container"><h2>Utilisateurs bloqués</h2>`
	if len(blockedUsers) == 0 {
		page += `<p>Aucun utilisateur bloqué.</p>`
	} else {
		page += `<div class="blocked-list">`
		for _, b := range blockedUsers {
			if user, ok := b.User.(*models.User); ok {
				page += fmt.Sprintf(`
        <div class="blocked-item">
          <h4>%s %s (@%s)</h4>
          <p>Bloqué le: %s</p>
          <button onclick="unblockUser(%d)">Débloquer</button>
        </div>`, user.FirstName, user.LastName, user.Username, b.CreatedAt.Format("02/01/2006 15:04"), user.ID)
			}
		}
		page += `</div>`
	}
	page += `
  <p><a href="/profile">← Retour au profil</a></p>
  <script>
    async function unblockUser(userId) {
      if (!confirm('Voulez-vous débloquer cet utilisateur ?')) return;
      try {
        const res = await fetch('/api/profile/' + userId + '/block', { method: 'DELETE' });
        if (res.ok) location.reload(); else alert('Erreur lors du déblocage');
      } catch { alert('Erreur lors du déblocage'); }
    }
  </script>
  <script src="/static/js/global-error-handler.js"></script>
  <script src="/static/js/user_status.js"></script>
  <script src="/static/js/navigation_active.js"></script>
  <script src="/static/js/notifications_unified.js"></script>
  <script src="/static/js/profile.js"></script>
  </div></body></html>`

	_, _ = w.Write([]byte(page))
}

/* --------- Notifications temps réel --------- */

// notifie un like/match.
func (h *ProfileHandlers) sendLikeNotification(fromUserID, toUserID int, isMatch bool) {
	if h.hub == nil {
		return
	}
	fromUser, err := h.profileService.GetUserByID(fromUserID)
	if err != nil {
		return
	}
	if isMatch {
		msgTo := WebSocketMessage{
			Type: "notification",
			Data: map[string]any{
				"type":          "match",
				"from_user_id":  fromUserID,
				"from_username": fromUser.Username,
				"to_user_id":    toUserID,
				"message":       fmt.Sprintf("Vous avez un nouveau match avec %s !", fromUser.Username),
			},
			Timestamp: time.Now(),
		}
		h.sendWebSocketToUser(toUserID, msgTo)

		if toUser, err := h.profileService.GetUserByID(toUserID); err == nil {
			msgBack := WebSocketMessage{
				Type: "notification",
				Data: map[string]any{
					"type":          "match",
					"from_user_id":  toUserID,
					"from_username": toUser.Username,
					"to_user_id":    fromUserID,
					"message":       fmt.Sprintf("Vous avez un nouveau match avec %s !", toUser.Username),
				},
				Timestamp: time.Now(),
			}
			h.sendWebSocketToUser(fromUserID, msgBack)
		}
		return
	}
	likeMessage := WebSocketMessage{
		Type: "notification",
		Data: map[string]any{
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

// notifie un unlike.
func (h *ProfileHandlers) sendUnlikeNotification(fromUserID, toUserID int) {
	if h.hub == nil {
		return
	}
	fromUser, err := h.profileService.GetUserByID(fromUserID)
	if err != nil {
		return
	}
	wsMessage := WebSocketMessage{
		Type: "notification",
		Data: map[string]any{
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

// envoie un message WS au user.
func (h *ProfileHandlers) sendWebSocketToUser(userID int, message WebSocketMessage) {
	data, err := json.Marshal(message)
	if err != nil || h.hub == nil {
		return
	}
	if client, ok := h.hub.Clients[userID]; ok {
		select {
		case client.Send <- data:
		default:
			close(client.Send)
			delete(h.hub.Clients, userID)
		}
	}
}

/* --------- Utilitaires --------- */

// libellé relatif FR.
func formatRelativeTime(t time.Time) string {
	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "Il y a moins d'une minute"
	case diff < time.Hour:
		m := int(diff.Minutes())
		if m == 1 {
			return "Il y a 1 minute"
		}
		return fmt.Sprintf("Il y a %d minutes", m)
	case diff < 24*time.Hour:
		h := int(diff.Hours())
		if h == 1 {
			return "Il y a 1 heure"
		}
		return fmt.Sprintf("Il y a %d heures", h)
	case diff < 7*24*time.Hour:
		d := int(diff.Hours() / 24)
		if d == 1 {
			return "Il y a 1 jour"
		}
		return fmt.Sprintf("Il y a %d jours", d)
	default:
		return t.Format("02/01/2006")
	}
}

// retourne l’IP client (en tenant compte des proxys).
func getClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		ips := strings.Split(ip, ",")
		return strings.TrimSpace(ips[0])
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Client-IP"); ip != "" {
		return ip
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// renvoie une géoloc approximative (fake/dev).
func (h *ProfileHandlers) IPGeolocationHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(r); !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return
	}
	clientIP := getClientIP(r)
	_ = clientIP
	resp := GeolocationResponse{Latitude: 48.8566, Longitude: 2.3522, City: "Paris", Country: "France", Method: "ip_approximation", Accuracy: 10000}
	writeOK(w, resp)
}

// retourne "selected" si condition vraie.
func getSelected(cond bool) string {
	if cond {
		return "selected"
	}
	return ""
}

// retourne "disabled" si condition vraie.
func getDisabled(cond bool) string {
	if cond {
		return "disabled"
	}
	return ""
}

// en ISO-8601 pour input date.
func formatBirthDate(b *time.Time) string {
	if b == nil {
		return ""
	}
	return b.Format("2006-01-02")
}

// rend les tags éditables (HTML).
func renderTags(tags []Tag) string {
	if len(tags) == 0 {
		return "<p>Aucun intérêt ajouté</p>"
	}
	var sb strings.Builder
	for _, t := range tags {
		sb.WriteString(fmt.Sprintf(`<span class="tag">%s <button class="remove-tag" data-id="%d">×</button></span>`, html.EscapeString(t.Name), t.ID))
	}
	return sb.String()
}

// rend les photos éditables (HTML).
func renderPhotos(photos []Photo) string {
	if len(photos) == 0 {
		return "<p>Aucune photo ajoutée</p>"
	}
	var sb strings.Builder
	for _, p := range photos {
		profileClass := ""
		if p.IsProfile {
			profileClass = "profile-photo"
		}
		sb.WriteString(fmt.Sprintf(`
<div class="photo-container %s" data-id="%d">
  <img src="%s" alt="Photo" style="max-width:200px;max-height:200px;">
  <div class="photo-actions">
    <button class="set-profile-photo" %s>Définir comme photo de profil</button>
    <button class="delete-photo">Supprimer</button>
  </div>
</div>`, profileClass, p.ID, p.FilePath, getDisabled(p.IsProfile)))
	}
	return sb.String()
}

// rend les photos (lecture seule).
func renderUserPhotos(photos []Photo) string {
	if len(photos) == 0 {
		return `<div class="no-photos"><img src="/static/images/default-profile.jpg" alt="Pas de photo" style="max-width:300px;border-radius:8px;"></div>`
	}
	var sb strings.Builder
	sb.WriteString(`<div class="user-photos">`)
	for _, p := range photos {
		profileClass := ""
		if p.IsProfile {
			profileClass = "profile-photo"
		}
		sb.WriteString(fmt.Sprintf(`<div class="user-photo %s"><img src="%s" alt="Photo" style="max-width:200px;max-height:200px;margin:5px;border-radius:8px;"></div>`, profileClass, p.FilePath))
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// rend les tags (lecture seule).
func renderUserTags(tags []Tag) string {
	if len(tags) == 0 {
		return "<p>Aucun intérêt spécifié</p>"
	}
	var sb strings.Builder
	sb.WriteString(`<div class="user-tags">`)
	for _, t := range tags {
		sb.WriteString(fmt.Sprintf(`<span class="tag">%s</span>`, html.EscapeString(t.Name)))
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// rend le bouton like/unlike.
func renderLikeButton(userID int, liked bool, matched bool) string {
	if liked {
		return fmt.Sprintf(`<button onclick="unlikeUser(%d)" class="unlike-button" type="button">💔 Ne plus liker</button>`, userID)
	}
	return fmt.Sprintf(`<button onclick="likeUser(%d)" class="like-button" type="button">👍 Liker</button>`, userID)
}

// rend le bouton chat s’il y a match.
func renderChatButton(userID int, matched bool) string {
	if matched {
		return fmt.Sprintf(`<button onclick="openChat(%d)" class="chat-button" type="button">💬 Discuter</button>`, userID)
	}
	return ""
}

// gère la requête pour obtenir le statut en ligne d'un utilisateur.
func (h *ProfileHandlers) GetUserOnlineStatusHandler(w http.ResponseWriter, r *http.Request) {
	h.GetUserStatusHandler(w, r)
}

// notifie une consultation de profil.
func (h *ProfileHandlers) sendProfileViewNotification(viewedUserID, viewerID int) {
	if h.hub == nil {
		return
	}
	viewer, err := h.profileService.GetUserByID(viewerID)
	if err != nil {
		return
	}
	wsMessage := WebSocketMessage{
		Type: "notification",
		Data: map[string]any{
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
