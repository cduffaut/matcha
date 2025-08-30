package user

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/cduffaut/matcha/internal/models"
)

/* ------------ Service ------------ */

type BrowsingService struct {
	userRepo    Repository
	profileRepo ProfileRepository
}

func NewBrowsingService(userRepo Repository, profileRepo ProfileRepository) *BrowsingService {
	return &BrowsingService{userRepo: userRepo, profileRepo: profileRepo}
}

/* ------------ Modèles ------------ */

type SuggestedProfileResult struct {
	Profile            *Profile
	User               *models.User
	CompatibilityScore float64
	Distance           float64
	CommonTags         int
	Age                int
}

/* ------------ Constantes / helpers ------------ */

const (
	maxDistanceForScore = 100.0
	maxCommonTagsForScore = 5.0
	maxFameRatingForScore = 100.0
)

func zoneForDistance(d float64) int {
	switch {
	case d < 180:
		return 1
	case d < 250:
		return 2
	case d < 350:
		return 3
	case d < 500:
		return 4
	default:
		return 5
	}
}

func paginate(results []SuggestedProfileResult, limit, offset int) []SuggestedProfileResult {
	if offset >= len(results) {
		return []SuggestedProfileResult{}
	}
	end := offset + limit
	if end > len(results) {
		end = len(results)
	}
	return results[offset:end]
}

func calculateAge(birthDate time.Time) int {
	now := time.Now()
	age := now.Year() - birthDate.Year()
	if now.YearDay() < birthDate.YearDay() {
		age--
	}
	return age
}

/* ------------ Règles de complétude / compatibilité ------------ */

func (s *BrowsingService) IsProfileComplete(p *Profile) bool {
	if p.Gender == "" || p.SexualPreference == "" || strings.TrimSpace(p.Biography) == "" || p.BirthDate == nil || len(p.Tags) == 0 {
		return false
	}
	for _, ph := range p.Photos {
		if ph.IsProfile {
			return true
		}
	}
	return false
}

func isCompatibleOrientation(p1, p2 *Profile) bool {
	pref1, pref2 := p1.SexualPreference, p2.SexualPreference
	if pref1 == "" {
		pref1 = PrefBisexual
	}
	if pref2 == "" {
		pref2 = PrefBisexual
	}
	g1, g2 := p1.Gender, p2.Gender

	switch pref1 {
	case PrefBisexual:
		switch pref2 {
		case PrefBisexual:
			return true
		case PrefHeterosexual:
			return g1 != g2
		case PrefHomosexual:
			return g1 == g2
		}
	case PrefHeterosexual:
		return g1 != g2 && (pref2 == PrefBisexual || pref2 == PrefHeterosexual)
	case PrefHomosexual:
		return g1 == g2 && (pref2 == PrefBisexual || pref2 == PrefHomosexual)
	}
	return false
}

/* ------------ Calculs ------------ */

func countCommonTags(a, b []Tag) int {
	set := make(map[string]struct{}, len(a))
	for _, t := range a {
		set[t.Name] = struct{}{}
	}
	n := 0
	for _, t := range b {
		if _, ok := set[t.Name]; ok {
			n++
		}
	}
	return n
}

func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	lat1r, lon1r := lat1*math.Pi/180, lon1*math.Pi/180
	lat2r, lon2r := lat2*math.Pi/180, lon2*math.Pi/180
	dlon, dlat := lon2r-lon1r, lat2r-lat1r
	a := math.Pow(math.Sin(dlat/2), 2) + math.Cos(lat1r)*math.Cos(lat2r)*math.Pow(math.Sin(dlon/2), 2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

/* ------------ Suggestions ------------ */

func (s *BrowsingService) GetSuggestions(userID, limit, offset int) ([]SuggestedProfileResult, error) {
	me, err := s.profileRepo.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération du profil: %w", err)
	}
	if !s.IsProfileComplete(me) {
		return []SuggestedProfileResult{}, fmt.Errorf("votre profil doit être complété pour voir des suggestions")
	}

	all, err := s.profileRepo.GetAllProfiles()
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des profils: %w", err)
	}

	results := make([]SuggestedProfileResult, 0, len(all))
	for _, p := range all {
		if p.UserID == userID || !s.IsProfileComplete(p) {
			continue
		}
		if ok, _ := s.profileRepo.IsBlocked(userID, p.UserID); ok {
			continue
		}
		if ok, _ := s.profileRepo.CheckIfLiked(userID, p.UserID); ok {
			continue
		}
		if ok, _ := s.profileRepo.CheckIfMatched(userID, p.UserID); ok {
			continue
		}
		if !isCompatibleOrientation(me, p) {
			continue
		}

		u, err := s.userRepo.GetByID(p.UserID)
		if err != nil {
			continue
		}

		dist := calculateDistance(me.Latitude, me.Longitude, p.Latitude, p.Longitude)
		common := countCommonTags(me.Tags, p.Tags)

		distanceScore := math.Max(0, 1.0-dist/maxDistanceForScore)
		tagsScore := math.Min(1.0, float64(common)/maxCommonTagsForScore)
		fameScore := float64(p.FameRating) / maxFameRatingForScore
		score := distanceScore*0.5 + tagsScore*0.3 + fameScore*0.2

		age := 0
		if p.BirthDate != nil {
			age = calculateAge(*p.BirthDate)
		}

		results = append(results, SuggestedProfileResult{
			Profile:            p,
			User:               u,
			CompatibilityScore: score,
			Distance:           dist,
			CommonTags:         common,
			Age:                age,
		})
	}

	// Tri par zone géographique (priorité) puis par score décroissant
	sort.Slice(results, func(i, j int) bool {
		zi, zj := zoneForDistance(results[i].Distance), zoneForDistance(results[j].Distance)
		if zi != zj {
			return zi < zj
		}
		return results[i].CompatibilityScore > results[j].CompatibilityScore
	})

	return paginate(results, limit, offset), nil
}

/* ------------ Recherche ------------ */

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

func (s *BrowsingService) SearchProfiles(userID int, opt FilterOptions, limit, offset int) ([]SuggestedProfileResult, error) {
	me, err := s.profileRepo.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération du profil: %w", err)
	}
	if !s.IsProfileComplete(me) {
		return []SuggestedProfileResult{}, fmt.Errorf("votre profil doit être complété pour effectuer des recherches")
	}

	// Base: suggestions déjà filtrées (blocage/like/match/orientation/complétude) + score/distance calculés
	suggestions, err := s.GetSuggestions(userID, 1000, 0)
	if err != nil {
		return nil, err
	}

	// Pré-normalisation des tags recherchés (accepte avec/sans #)
	wanted := make(map[string]struct{}, len(opt.Tags)*2)
	for _, t := range opt.Tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if !strings.HasPrefix(t, "#") {
			wanted["#"+t] = struct{}{}
		}
		wanted[t] = struct{}{}
	}

	filtered := make([]SuggestedProfileResult, 0, len(suggestions))
	for _, sgg := range suggestions {
		if opt.MinFame > 0 && sgg.Profile.FameRating < opt.MinFame {
			continue
		}
		if opt.MaxFame > 0 && sgg.Profile.FameRating > opt.MaxFame {
			continue
		}
		if opt.MaxDistance > 0 && sgg.Distance > opt.MaxDistance {
			continue
		}
		if opt.MinAge > 0 && sgg.Age < opt.MinAge {
			continue
		}
		if opt.MaxAge > 0 && sgg.Age > opt.MaxAge {
			continue
		}

		if len(wanted) > 0 {
			hasAll := true
			have := make(map[string]struct{}, len(sgg.Profile.Tags)*2)
			for _, t := range sgg.Profile.Tags {
				have[t.Name] = struct{}{}
				if !strings.HasPrefix(t.Name, "#") {
					have["#"+t.Name] = struct{}{}
				} else {
					have[strings.TrimPrefix(t.Name, "#")] = struct{}{}
				}
			}
			for k := range wanted {
				if _, ok := have[k]; !ok {
					hasAll = false
					break
				}
			}
			if !hasAll {
				continue
			}
		}

		filtered = append(filtered, sgg)
	}

	asc := strings.ToLower(opt.SortOrder) == "asc"
	switch opt.SortBy {
	case "age":
		sort.Slice(filtered, func(i, j int) bool {
			if asc {
				return filtered[i].Age < filtered[j].Age
			}
			return filtered[i].Age > filtered[j].Age
		})
	case "distance":
		sort.Slice(filtered, func(i, j int) bool {
			if asc {
				return filtered[i].Distance < filtered[j].Distance
			}
			return filtered[i].Distance > filtered[j].Distance
		})
	case "fame":
		sort.Slice(filtered, func(i, j int) bool {
			if asc {
				return filtered[i].Profile.FameRating < filtered[j].Profile.FameRating
			}
			return filtered[i].Profile.FameRating > filtered[j].Profile.FameRating
		})
	case "common_tags":
		sort.Slice(filtered, func(i, j int) bool {
			if asc {
				return filtered[i].CommonTags < filtered[j].CommonTags
			}
			return filtered[i].CommonTags > filtered[j].CommonTags
		})
	default:
		sort.Slice(filtered, func(i, j int) bool {
			if asc {
				return filtered[i].CompatibilityScore < filtered[j].CompatibilityScore
			}
			return filtered[i].CompatibilityScore > filtered[j].CompatibilityScore
		})
	}

	return paginate(filtered, limit, offset), nil
}
