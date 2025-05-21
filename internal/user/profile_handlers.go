package user

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cduffaut/matcha/internal/session"
	"goji.io/pat"
)

// ProfileHandlers gère les requêtes HTTP pour les profils utilisateurs
type ProfileHandlers struct {
	profileService *ProfileService
}

// NewProfileHandlers crée de nouveaux gestionnaires pour les profils
func NewProfileHandlers(profileService *ProfileService) *ProfileHandlers {
	return &ProfileHandlers{
		profileService: profileService,
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

// UpdateProfileHandler met à jour le profil de l'utilisateur connecté
func (h *ProfileHandlers) UpdateProfileHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Décoder le corps de la requête
	var req ProfileUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Format de requête invalide", http.StatusBadRequest)
		return
	}

	// Récupérer le profil existant
	profile, err := h.profileService.GetProfile(session.UserID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération du profil", http.StatusInternalServerError)
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
		http.Error(w, "Erreur lors de la mise à jour du profil", http.StatusInternalServerError)
		return
	}

	// Mettre à jour les tags si nécessaire
	if req.Tags != nil {
		// Supprimer les anciens tags
		for _, tag := range profile.Tags {
			if err := h.profileService.RemoveTag(session.UserID, tag.ID); err != nil {
				// On continue malgré les erreurs
				fmt.Printf("Erreur lors de la suppression du tag: %v\n", err)
			}
		}

		// Ajouter les nouveaux tags
		for _, tagName := range req.Tags {
			if err := h.profileService.AddTag(session.UserID, tagName); err != nil {
				// On continue malgré les erreurs
				fmt.Printf("Erreur lors de l'ajout du tag: %v\n", err)
			}
		}
	}

	// Récupérer le profil mis à jour
	updatedProfile, err := h.profileService.GetProfile(session.UserID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération du profil mis à jour", http.StatusInternalServerError)
		return
	}

	// Dans UpdateProfileHandler, vérifiez que la date de naissance est correctement reçue
	fmt.Printf("Date de naissance reçue: %v\n", req.BirthDate)

	// Répondre avec le profil mis à jour
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedProfile)
}

// AddTagHandler ajoute un tag au profil de l'utilisateur connecté
func (h *ProfileHandlers) AddTagHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Décoder le corps de la requête
	var req struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Format de requête invalide", http.StatusBadRequest)
		return
	}

	// Ajouter le tag
	if err := h.profileService.AddTag(session.UserID, req.TagName); err != nil {
		http.Error(w, "Erreur lors de l'ajout du tag", http.StatusInternalServerError)
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

// UploadPhotoHandler télécharge une photo pour l'utilisateur connecté
func (h *ProfileHandlers) UploadPhotoHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Vérifier que le formulaire est multipart
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Format de requête invalide", http.StatusBadRequest)
		return
	}

	// Récupérer le fichier
	file, header, err := r.FormFile("photo")
	if err != nil {
		http.Error(w, "Erreur lors de la récupération du fichier", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Vérifier le type de fichier
	if !strings.HasPrefix(header.Header.Get("Content-Type"), "image/") {
		http.Error(w, "Type de fichier non autorisé", http.StatusBadRequest)
		return
	}

	// Lire le fichier
	fileData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Erreur lors de la lecture du fichier", http.StatusInternalServerError)
		return
	}

	// Vérifier si c'est une photo de profil
	isProfile := r.FormValue("is_profile") == "true"

	// Télécharger la photo
	photo, err := h.profileService.UploadPhoto(session.UserID, fileData, header.Filename, isProfile)
	if err != nil {
		http.Error(w, fmt.Sprintf("Erreur lors du téléchargement de la photo: %v", err), http.StatusInternalServerError)
		return
	}

	// Répondre avec la photo
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(photo)
}

// DeletePhotoHandler supprime une photo de l'utilisateur connecté
func (h *ProfileHandlers) DeletePhotoHandler(w http.ResponseWriter, r *http.Request) {
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

	// Supprimer la photo
	if err := h.profileService.DeletePhoto(session.UserID, photoID); err != nil {
		http.Error(w, fmt.Sprintf("Erreur lors de la suppression de la photo: %v", err), http.StatusInternalServerError)
		return
	}

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Photo supprimée avec succès",
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

	// Enregistrer la visite
	if err := h.profileService.ViewProfile(session.UserID, userID); err != nil {
		// On continue malgré les erreurs
		fmt.Printf("Erreur lors de l'enregistrement de la visite: %v\n", err)
	}

	// Récupérer le profil
	profile, err := h.profileService.GetProfile(userID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération du profil", http.StatusInternalServerError)
		return
	}

	// Vérifier si l'utilisateur a liké ce profil
	liked, err := h.profileService.CheckIfLiked(session.UserID, userID)
	if err != nil {
		// On continue malgré les erreurs
		fmt.Printf("Erreur lors de la vérification du like: %v\n", err)
	}

	// Vérifier s'il y a un match
	matched, err := h.profileService.CheckIfMatched(session.UserID, userID)
	if err != nil {
		// On continue malgré les erreurs
		fmt.Printf("Erreur lors de la vérification du match: %v\n", err)
	}

	// Répondre avec le profil et les informations supplémentaires
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"profile": profile,
		"liked":   liked,
		"matched": matched,
	})
}

// GetVisitorsHandler récupère les visiteurs du profil de l'utilisateur connecté
func (h *ProfileHandlers) GetVisitorsHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer les visiteurs
	visitors, err := h.profileService.GetVisitors(session.UserID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des visiteurs", http.StatusInternalServerError)
		return
	}

	// Répondre avec les visiteurs
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(visitors)
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
		http.Error(w, fmt.Sprintf("Erreur lors du like: %v", err), http.StatusInternalServerError)
		return
	}

	// Répondre avec le statut du match
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Utilisateur liké avec succès",
		"matched": matched,
	})
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

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Utilisateur unliké avec succès",
	})
}

// GetLikesHandler récupère les likes reçus par l'utilisateur connecté
func (h *ProfileHandlers) GetLikesHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer les likes
	likes, err := h.profileService.GetLikes(session.UserID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des likes", http.StatusInternalServerError)
		return
	}

	// Répondre avec les likes
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(likes)
}

func getSelected(condition bool) string {
	if condition {
		return "selected"
	}
	return ""
}

func formatBirthDate(birthDate *time.Time) string {
	if birthDate == nil {
		return ""
	}
	return birthDate.Format("2006-01-02")
}

// ProfilePageHandler affiche la page de profil
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

	// Afficher la page de profil
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	fmt.Fprintf(w, `
        <!DOCTYPE html>
        <html lang="fr">
        <head>
            <title>Profil - Matcha</title>
            <meta charset="UTF-8">
            <meta name="viewport" content="width=device-width, initial-scale=1">
            <link rel="stylesheet" href="/static/css/profile.css?v=%d">
        </head>
        <body>
            <header>
                <h1>Matcha</h1>
                <nav>
                    <a href="/profile" class="active">Mon profil</a>
                    <a href="/browse">Explorer</a>
                    <a href="/chat">Messages</a>
                    <a href="/logout">Déconnexion</a>
                </nav>
            </header>
            
            <div class="container">
                <h2>Mon profil</h2>
                
                <div id="profile-form">
                    <div class="form-group">
                        <label for="gender">Genre</label>
                        <select id="gender" name="gender">
                            <option value="" %s>Non spécifié</option>
                            <option value="male" %s>Homme</option>
                            <option value="female" %s>Femme</option>
                            <option value="other" %s>Autre</option>
                        </select>
                    </div>
                    
                    <div class="form-group">
                        <label for="sexual_preference">Préférence sexuelle</label>
                        <select id="sexual_preference" name="sexual_preference">
                            <option value="" %s>Non spécifié</option>
                            <option value="heterosexual" %s>Hétérosexuel</option>
                            <option value="homosexual" %s>Homosexuel</option>
                            <option value="bisexual" %s>Bisexuel</option>
                        </select>
                    </div>
                    
                    <div class="form-group">
                        <label for="biography">Biographie</label>
                        <textarea id="biography" name="biography" placeholder="Parlez-nous de vous...">%s</textarea>
                    </div>
					
					<div class="form-group">
    					<label for="birth_date">Date de naissance</label>
    					<input type="date" id="birth_date" name="birth_date" value="%s">
					</div>
                    
                    <div class="form-group">
                        <label for="location">Localisation</label>
                        <input type="text" id="location" name="location" value="%s" readonly>
                        <button type="button" id="update-location">Mettre à jour ma position</button>
                    </div>
                    
                    <div class="form-group">
                        <label>Intérêts</label>
                        <div id="tags-container">
                            %s
                        </div>
                        <div id="add-tag">
                            <input type="text" id="new-tag" placeholder="Nouvel intérêt (ex: #sport)">
                            <button type="button" id="add-tag-btn">Ajouter</button>
                        </div>
                    </div>
                    
                    <button type="button" id="save-profile">Enregistrer les modifications</button>
                </div>
                
                <div id="photos-section">
                    <h3>Mes photos</h3>
                    <div id="photos-container">
                        %s
                    </div>
                    <div id="upload-photo">
                        <form id="photo-form" enctype="multipart/form-data">
                            <input type="file" id="photo-input" name="photo" accept="image/*">
                            <label for="is-profile">
                                <input type="checkbox" id="is-profile" name="is_profile">
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
                </div>
            </div>
            
            <script src="/static/js/profile.js?v=%d"></script>
        </body>
        </html>
    `,
		time.Now().Unix(), // Version pour forcer le rechargement du CSS
		getSelected(profile.Gender == ""),
		getSelected(profile.Gender == "male"),
		getSelected(profile.Gender == "female"),
		getSelected(profile.Gender == "other"),
		getSelected(profile.SexualPreference == ""),
		getSelected(profile.SexualPreference == "heterosexual"),
		getSelected(profile.SexualPreference == "homosexual"),
		getSelected(profile.SexualPreference == "bisexual"),
		profile.Biography,
		formatBirthDate(profile.BirthDate),
		profile.LocationName,
		renderTags(profile.Tags),
		renderPhotos(profile.Photos),
		profile.FameRating,
		time.Now().Unix()) // Version pour forcer le rechargement du JS
}

// Fonction d'aide pour rendre les tags en HTML
func renderTags(tags []Tag) string {
	html := ""
	for _, tag := range tags {
		html += fmt.Sprintf(`<span class="tag">%s <button class="remove-tag" data-id="%d">×</button></span>`, tag.Name, tag.ID)
	}
	return html
}

func getDisabled(condition bool) string {
	if condition {
		return "disabled"
	}
	return ""
}

// Fonction d'aide pour rendre les photos en HTML
func renderPhotos(photos []Photo) string {
	html := ""
	for _, photo := range photos {
		profileClass := ""
		if photo.IsProfile {
			profileClass = "profile-photo"
		}
		html += fmt.Sprintf(`
            <div class="photo-container %s" data-id="%d">
                <img src="%s" alt="Photo">
                <div class="photo-actions">
                    <button class="set-profile-photo" %s>Définir comme photo de profil</button>
                    <button class="delete-photo">Supprimer</button>
                </div>
            </div>
        `, profileClass, photo.ID, photo.FilePath, getDisabled(photo.IsProfile))
	}
	return html
}
