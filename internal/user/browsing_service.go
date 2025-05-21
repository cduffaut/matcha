package user

import (
	"fmt"
	"math"
	"sort"
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
}

// GetSuggestions récupère des suggestions de profils selon des critères
func (s *BrowsingService) GetSuggestions(userID int, limit, offset int) ([]SuggestedProfileResult, error) {
	// Récupérer le profil de l'utilisateur
	currentProfile, err := s.profileRepo.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération du profil: %w", err)
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

		// Ajouter à la liste
		filteredProfiles = append(filteredProfiles, SuggestedProfileResult{
			Profile:            profile,
			User:               user,
			CompatibilityScore: compatibilityScore,
			Distance:           distance,
			CommonTags:         commonTags,
		})
	}

	// Trier selon le score de compatibilité
	sort.Slice(filteredProfiles, func(i, j int) bool {
		return filteredProfiles[i].CompatibilityScore > filteredProfiles[j].CompatibilityScore
	})

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

		if options.MinAge > 0 || options.MaxAge > 0 {
			var age int
			if suggestion.Profile.BirthDate != nil {
				age = calculateAge(*suggestion.Profile.BirthDate)

				if options.MinAge > 0 && age < options.MinAge {
					continue
				}
				if options.MaxAge > 0 && age > options.MaxAge {
					continue
				}
			} else if options.MinAge > 0 {
				// Si l'âge minimum est défini et que l'utilisateur n'a pas de date de naissance,
				// on peut choisir d'exclure ce profil ou non selon la politique souhaitée
				continue
			}
		}

		// Filtrer par tags
		if len(options.Tags) > 0 {
			hasAllTags := true
			tagMap := make(map[string]bool)

			for _, tag := range suggestion.Profile.Tags {
				tagMap[tag.Name] = true
			}

			for _, tag := range options.Tags {
				if !tagMap[tag] {
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
			// Obtenir les âges (0 si pas de date de naissance)
			ageI := 0
			if filtered[i].Profile.BirthDate != nil {
				ageI = calculateAge(*filtered[i].Profile.BirthDate)
			}

			ageJ := 0
			if filtered[j].Profile.BirthDate != nil {
				ageJ = calculateAge(*filtered[j].Profile.BirthDate)
			}

			if options.SortOrder == "asc" {
				return ageI < ageJ
			}
			return ageI > ageJ
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
