package user

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/cduffaut/matcha/internal/models"
)

// BrowsingService fournit des services pour explorer les profils
type BrowsingService struct {
	userRepo    Repository
	profileRepo ProfileRepository
}

// NewBrowsingService crée un nouveau service de browsing
func NewBrowsingService(userRepo Repository, profileRepo ProfileRepository) *BrowsingService {
	return &BrowsingService{
		userRepo:    userRepo,
		profileRepo: profileRepo,
	}
}

// SuggestedProfileResult représente un profil suggéré avec un score de compatibilité
type SuggestedProfileResult struct {
	Profile            *Profile
	User               *models.User
	CompatibilityScore float64
	Distance           float64
	CommonTags         int
	Age                int
}

// IsProfileComplete vérifie si un profil remplit toutes les conditions obligatoires
func (s *BrowsingService) IsProfileComplete(profile *Profile) bool {
	// 1. Genre obligatoire
	if profile.Gender == "" {
		return false
	}

	// 2. Préférence sexuelle obligatoire
	if profile.SexualPreference == "" {
		return false
	}

	// 3. Biographie obligatoire (non vide après trim)
	if strings.TrimSpace(profile.Biography) == "" {
		return false
	}

	// 4. Date de naissance obligatoire
	if profile.BirthDate == nil {
		return false
	}

	// 5. Au moins un tag/intérêt obligatoire
	if len(profile.Tags) == 0 {
		return false
	}

	// 6. Au moins une photo de profil obligatoire
	hasProfilePhoto := false
	for _, photo := range profile.Photos {
		if photo.IsProfile {
			hasProfilePhoto = true
			break
		}
	}
	if !hasProfilePhoto {
		return false
	}

	return true
}

// GetSuggestions récupère des suggestions de profils selon des critères
func (s *BrowsingService) GetSuggestions(userID int, limit, offset int) ([]SuggestedProfileResult, error) {
	// Récupérer le profil de l'utilisateur
	currentProfile, err := s.profileRepo.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération du profil: %w", err)
	}

	// AJOUT: Vérifier que le profil actuel est complet AVANT de chercher des suggestions
	if !s.IsProfileComplete(currentProfile) {
		return []SuggestedProfileResult{}, fmt.Errorf("votre profil doit être complété pour voir des suggestions")
	}

	currentUser, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération de l'utilisateur: %w", err)
	}

	// Récupérer tous les profils
	profiles, err := s.profileRepo.GetAllProfiles()
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des profils: %w", err)
	}

	// Filtrer les profils selon les préférences
	var filteredProfiles []SuggestedProfileResult

	for _, profile := range profiles {
		// Ignorer son propre profil
		if profile.UserID == userID {
			continue
		}

		// AJOUT: Vérifier que le profil candidat est complet
		if !s.IsProfileComplete(profile) {
			continue
		}

		// Récupérer les infos de l'utilisateur
		user, err := s.userRepo.GetByID(profile.UserID)
		if err != nil {
			continue
		}

		// Filtrer selon l'orientation sexuelle
		if !s.isCompatibleOrientation(currentProfile, profile, currentUser, user) {
			continue
		}

		// Bloquer les utilisateurs qui ont bloqué l'utilisateur actuel ou que l'utilisateur actuel a bloqué
		blocked, err := s.profileRepo.IsBlocked(userID, profile.UserID)
		if err != nil || blocked {
			continue
		}

		alreadyLiked, err := s.profileRepo.CheckIfLiked(userID, profile.UserID)
		if err != nil || alreadyLiked {
			continue
		}

		// CORRECTION: Exclure les utilisateurs déjà matchés
		matched, err := s.profileRepo.CheckIfMatched(userID, profile.UserID)
		if err != nil {
			continue // En cas d'erreur, ignorer ce profil
		}
		if matched {
			continue // Ignorer les profils déjà matchés
		}

		// Calculer la distance
		distance := calculateDistance(
			currentProfile.Latitude, currentProfile.Longitude,
			profile.Latitude, profile.Longitude,
		)

		// Calculer les tags communs
		commonTags := s.countCommonTags(currentProfile.Tags, profile.Tags)

		// Calculer le score de compatibilité
		// 50% basé sur la distance, 30% sur les tags communs, 20% sur le fame rating
		maxDistance := 100.0 // km
		distanceScore := math.Max(0, 1.0-distance/maxDistance)
		maxCommonTags := 5
		tagsScore := float64(commonTags) / float64(maxCommonTags)
		maxFameRating := 100
		fameScore := float64(profile.FameRating) / float64(maxFameRating)

		compatibilityScore := distanceScore*0.5 + tagsScore*0.3 + fameScore*0.2

		age := 0
		if profile.BirthDate != nil {
			age = calculateAge(*profile.BirthDate)
		}

		// Ajouter à la liste
		filteredProfiles = append(filteredProfiles, SuggestedProfileResult{
			Profile:            profile,
			User:               user,
			CompatibilityScore: compatibilityScore,
			Distance:           distance,
			CommonTags:         commonTags,
			Age:                age,
		})
	}

	// Trier selon le score de compatibilité
	sort.Slice(filteredProfiles, func(i, j int) bool {
		return filteredProfiles[i].CompatibilityScore > filteredProfiles[j].CompatibilityScore
	})

	// NOUVEAU : Prioriser la zone géographique selon le cahier des charges
	// Créer 5 groupes par distance pour une démonstration claire de la priorité géographique
	var zone1Profiles []SuggestedProfileResult // < 180km (très proche)
	var zone2Profiles []SuggestedProfileResult // 180-250km (proche)
	var zone3Profiles []SuggestedProfileResult // 250-350km (moyen)
	var zone4Profiles []SuggestedProfileResult // 350-500km (loin)
	var zone5Profiles []SuggestedProfileResult // > 500km (très loin)

	for _, profile := range filteredProfiles {
		if profile.Distance < 180.0 {
			zone1Profiles = append(zone1Profiles, profile)
		} else if profile.Distance < 250.0 {
			zone2Profiles = append(zone2Profiles, profile)
		} else if profile.Distance < 350.0 {
			zone3Profiles = append(zone3Profiles, profile)
		} else if profile.Distance < 500.0 {
			zone4Profiles = append(zone4Profiles, profile)
		} else {
			zone5Profiles = append(zone5Profiles, profile)
		}
	}

	// DEBUG: Afficher la répartition par zones (optionnel - à supprimer en production)
	fmt.Printf("DEBUG - Geographic zones: Zone1(<180km)=%d, Zone2(180-250km)=%d, Zone3(250-350km)=%d, Zone4(350-500km)=%d, Zone5(>500km)=%d\n",
		len(zone1Profiles), len(zone2Profiles), len(zone3Profiles), len(zone4Profiles), len(zone5Profiles))

	// Trier chaque zone par score de compatibilité
	sort.Slice(zone1Profiles, func(i, j int) bool {
		return zone1Profiles[i].CompatibilityScore > zone1Profiles[j].CompatibilityScore
	})
	sort.Slice(zone2Profiles, func(i, j int) bool {
		return zone2Profiles[i].CompatibilityScore > zone2Profiles[j].CompatibilityScore
	})
	sort.Slice(zone3Profiles, func(i, j int) bool {
		return zone3Profiles[i].CompatibilityScore > zone3Profiles[j].CompatibilityScore
	})
	sort.Slice(zone4Profiles, func(i, j int) bool {
		return zone4Profiles[i].CompatibilityScore > zone4Profiles[j].CompatibilityScore
	})
	sort.Slice(zone5Profiles, func(i, j int) bool {
		return zone5Profiles[i].CompatibilityScore > zone5Profiles[j].CompatibilityScore
	})

	// Reconstruire la liste avec priorité géographique stricte
	filteredProfiles = []SuggestedProfileResult{}
	filteredProfiles = append(filteredProfiles, zone1Profiles...) // Très proche en premier
	filteredProfiles = append(filteredProfiles, zone2Profiles...) // Puis proche
	filteredProfiles = append(filteredProfiles, zone3Profiles...) // Puis moyen
	filteredProfiles = append(filteredProfiles, zone4Profiles...) // Puis loin
	filteredProfiles = append(filteredProfiles, zone5Profiles...) // Très loin en dernier

	// Pagination
	start := offset
	end := offset + limit
	if start >= len(filteredProfiles) {
		return []SuggestedProfileResult{}, nil
	}
	if end > len(filteredProfiles) {
		end = len(filteredProfiles)
	}

	return filteredProfiles[start:end], nil
}

// isCompatibleOrientation vérifie si deux profils sont compatibles selon leur orientation
func (s *BrowsingService) isCompatibleOrientation(profile1, profile2 *Profile, user1, user2 *models.User) bool {
	// Si l'utilisateur n'a pas spécifié d'orientation, il est considéré bisexuel
	pref1 := profile1.SexualPreference
	if pref1 == "" {
		pref1 = PrefBisexual
	}

	pref2 := profile2.SexualPreference
	if pref2 == "" {
		pref2 = PrefBisexual
	}

	gender1 := profile1.Gender
	gender2 := profile2.Gender

	// Cas bisexuel
	if pref1 == PrefBisexual {
		// Compatible si l'autre est attiré par le genre de cet utilisateur
		if pref2 == PrefBisexual {
			return true
		}
		if pref2 == PrefHeterosexual {
			return gender1 != gender2
		}
		if pref2 == PrefHomosexual {
			return gender1 == gender2
		}
	}

	// Cas hétérosexuel
	if pref1 == PrefHeterosexual {
		if gender1 == gender2 {
			return false // Même genre, non compatible
		}
		// L'autre doit être bisexuel ou hétérosexuel
		return pref2 == PrefBisexual || pref2 == PrefHeterosexual
	}

	// Cas homosexuel
	if pref1 == PrefHomosexual {
		if gender1 != gender2 {
			return false // Genre différent, non compatible
		}
		// L'autre doit être bisexuel ou homosexuel
		return pref2 == PrefBisexual || pref2 == PrefHomosexual
	}

	return false
}

// countCommonTags compte le nombre de tags communs entre deux listes
func (s *BrowsingService) countCommonTags(tags1, tags2 []Tag) int {
	tagMap := make(map[string]bool)

	for _, tag := range tags1 {
		tagMap[tag.Name] = true
	}

	count := 0
	for _, tag := range tags2 {
		if tagMap[tag.Name] {
			count++
		}
	}

	return count
}

// calculateDistance calcule la distance en km entre deux points géographiques
func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0 // Rayon de la Terre en km

	lat1Rad := toRadians(lat1)
	lon1Rad := toRadians(lon1)
	lat2Rad := toRadians(lat2)
	lon2Rad := toRadians(lon2)

	dlon := lon2Rad - lon1Rad
	dlat := lat2Rad - lat1Rad

	a := math.Pow(math.Sin(dlat/2), 2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Pow(math.Sin(dlon/2), 2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

func toRadians(deg float64) float64 {
	return deg * math.Pi / 180
}

// FilterOptions contient les options de filtrage pour la recherche
type FilterOptions struct {
	MinAge      int
	MaxAge      int
	MinFame     int
	MaxFame     int
	MaxDistance float64
	Tags        []string
	SortBy      string // "age", "distance", "fame", "common_tags"
	SortOrder   string // "asc", "desc"
}

// SearchProfiles recherche des profils selon des critères
func (s *BrowsingService) SearchProfiles(userID int, options FilterOptions, limit, offset int) ([]SuggestedProfileResult, error) {
	// AJOUT: Vérifier que le profil actuel est complet AVANT de permettre la recherche
	currentProfile, err := s.profileRepo.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération du profil: %w", err)
	}

	if !s.IsProfileComplete(currentProfile) {
		return []SuggestedProfileResult{}, fmt.Errorf("votre profil doit être complété pour effectuer des recherches")
	}

	// Similaire à GetSuggestions mais avec des filtres supplémentaires
	suggestions, err := s.GetSuggestions(userID, 1000, 0) // Récupère un grand nombre
	if err != nil {
		return nil, err
	}

	var filtered []SuggestedProfileResult

	for _, suggestion := range suggestions {
		// Filtrer par critères
		if options.MinFame > 0 && suggestion.Profile.FameRating < options.MinFame {
			continue
		}
		if options.MaxFame > 0 && suggestion.Profile.FameRating > options.MaxFame {
			continue
		}
		if options.MaxDistance > 0 && suggestion.Distance > options.MaxDistance {
			continue
		}

		if options.MinAge > 0 && suggestion.Age < options.MinAge {
			continue
		}
		if options.MaxAge > 0 && suggestion.Age > options.MaxAge {
			continue
		}

		// Filtrer par tags
		if len(options.Tags) > 0 {
			hasAllTags := true
			tagMap := make(map[string]bool)

			// Construire une map avec TOUS les formats possibles des tags du profil
			for _, tag := range suggestion.Profile.Tags {
				// Ajouter le tag tel qu'il est stocké
				tagMap[tag.Name] = true

				// Ajouter aussi la version sans # si le tag commence par #
				if strings.HasPrefix(tag.Name, "#") {
					tagMap[tag.Name[1:]] = true
				} else {
					// Ajouter aussi la version avec # si le tag ne commence pas par #
					tagMap["#"+tag.Name] = true
				}
			}

			// Vérifier si tous les tags recherchés sont présents
			for _, tag := range options.Tags {
				// Normaliser le tag recherché : ajouter # si absent
				normalizedTag := tag
				if !strings.HasPrefix(tag, "#") {
					normalizedTag = "#" + tag
				}

				// Vérifier si le tag (avec ou sans #) existe dans la map
				if !tagMap[tag] && !tagMap[normalizedTag] {
					hasAllTags = false
					break
				}
			}

			if !hasAllTags {
				continue
			}
		}

		filtered = append(filtered, suggestion)
	}

	// Trier selon les critères
	switch options.SortBy {
	case "age":
		sort.Slice(filtered, func(i, j int) bool {
			if options.SortOrder == "asc" {
				return filtered[i].Age < filtered[j].Age
			}
			return filtered[i].Age > filtered[j].Age
		})
	case "distance":
		sort.Slice(filtered, func(i, j int) bool {
			if options.SortOrder == "asc" {
				return filtered[i].Distance < filtered[j].Distance
			}
			return filtered[i].Distance > filtered[j].Distance
		})
	case "fame":
		sort.Slice(filtered, func(i, j int) bool {
			if options.SortOrder == "asc" {
				return filtered[i].Profile.FameRating < filtered[j].Profile.FameRating
			}
			return filtered[i].Profile.FameRating > filtered[j].Profile.FameRating
		})
	case "common_tags":
		sort.Slice(filtered, func(i, j int) bool {
			if options.SortOrder == "asc" {
				return filtered[i].CommonTags < filtered[j].CommonTags
			}
			return filtered[i].CommonTags > filtered[j].CommonTags
		})
	default: // Par défaut, tri par compatibilité
		sort.Slice(filtered, func(i, j int) bool {
			if options.SortOrder == "asc" {
				return filtered[i].CompatibilityScore < filtered[j].CompatibilityScore
			}
			return filtered[i].CompatibilityScore > filtered[j].CompatibilityScore
		})
	}

	// Pagination
	start := offset
	end := offset + limit
	if start >= len(filtered) {
		return []SuggestedProfileResult{}, nil
	}
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[start:end], nil
}

// Fonction utilitaire pour calculer l'âge
func calculateAge(birthDate time.Time) int {
	today := time.Now()
	age := today.Year() - birthDate.Year()

	// Ajuster l'âge si l'anniversaire n'est pas encore passé cette année
	if today.YearDay() < birthDate.YearDay() {
		age--
	}

	return age
}
