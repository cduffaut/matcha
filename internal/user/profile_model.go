package user

import (
	"time"
)

// Préférences sexuelles
const (
	PrefHeterosexual = "heterosexual"
	PrefHomosexual   = "homosexual"
	PrefBisexual     = "bisexual"
)

// Gender représente le genre d'un utilisateur
type Gender string

// SexualPreference représente la préférence sexuelle d'un utilisateur
type SexualPreference string

// Profile représente le profil d'un utilisateur
type Profile struct {
	ID               int              `db:"id"`
	UserID           int              `db:"user_id"`
	FirstName        string           `db:"first_name"`
	LastName         string           `db:"last_name"`
	Gender           Gender           `db:"gender"`
	SexualPreference SexualPreference `db:"sexual_preference"`
	Biography        string           `db:"biography"`
	BirthDate        *time.Time       `db:"birth_date"`
	LocationName     string           `db:"location_name"`
	Latitude         float64          `db:"latitude"`
	Longitude        float64          `db:"longitude"`
	FameRating       int              `db:"fame_rating"`
	IsOnline         bool             `db:"is_online"`
	LastConnection   *time.Time       `db:"last_connection"`
	CreatedAt        time.Time        `db:"created_at"`
	UpdatedAt        time.Time        `db:"updated_at"`
	Photos           []Photo          `db:"-"`
	Tags             []Tag            `db:"-"`
}

// BlockedUser représente un utilisateur bloqué
type BlockedUser struct {
	ID        int         `db:"id"`
	BlockerID int         `db:"blocker_id"`
	BlockedID int         `db:"blocked_id"`
	CreatedAt time.Time   `db:"created_at"`
	User      interface{} `db:"-"` // Informations de l'utilisateur bloqué
}

// Tag représente un tag d'intérêt
type Tag struct {
	ID        int       `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
}

// Photo représente une photo de profil
type Photo struct {
	ID        int       `db:"id"`
	UserID    int       `db:"user_id"`
	FilePath  string    `db:"file_path"`
	IsProfile bool      `db:"is_profile"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// ProfileVisit représente une visite de profil
type ProfileVisit struct {
	ID        int         `db:"id"`
	VisitorID int         `db:"visitor_id"`
	VisitedID int         `db:"visited_id"`
	VisitedAt time.Time   `db:"visited_at"`
	Visitor   interface{} `db:"-"`
}

// UserLike représente un "like" entre deux utilisateurs
type UserLike struct {
	ID        int         `db:"id"`
	LikerID   int         `db:"liker_id"`
	LikedID   int         `db:"liked_id"`
	CreatedAt time.Time   `db:"created_at"`
	Liker     interface{} `db:"-"`
}

// ReportData représente un signalement avec informations détaillées
type ReportData struct {
	ID           int         `json:"id"`
	ReporterID   int         `json:"reporter_id"`
	ReportedID   int         `json:"reported_id"`
	Reason       string      `json:"reason"`
	CreatedAt    time.Time   `json:"created_at"`
	IsProcessed  bool        `json:"is_processed"`
	ProcessedAt  *time.Time  `json:"processed_at"`
	AdminComment string      `json:"admin_comment"`
	ReporterInfo interface{} `json:"reporter_info"`
	ReportedInfo interface{} `json:"reported_info"`
}

// ProfileRepository est l'interface pour accéder aux données des profils
type ProfileRepository interface {
	GetByUserID(userID int) (*Profile, error)
	Create(profile *Profile) error
	Update(profile *Profile) error
	GetTagsByUserID(userID int) ([]Tag, error)
	AddTag(userID int, tagName string) error
	RemoveTag(userID int, tagID int) error
	GetAllTags() ([]Tag, error)
	GetTagByID(tagID int) (*Tag, error)
	GetPhotosByUserID(userID int) ([]Photo, error)
	AddPhoto(photo *Photo) error
	RemovePhoto(photoID int) error
	SetProfilePhoto(photoID int) error
	IsProfilePhoto(userID int, photoID int) (bool, error)
	RecordVisit(visitorID, visitedID int) error
	GetVisitorsForUser(userID int) ([]ProfileVisit, error)
	LikeUser(likerID, likedID int) error
	UnlikeUser(likerID, likedID int) error
	GetLikesForUser(userID int) ([]UserLike, error)
	CheckIfLiked(likerID, likedID int) (bool, error)
	CheckIfMatched(user1ID, user2ID int) (bool, error)
	IsBlocked(userID, blockedID int) (bool, error)
	ReportUser(reporterID, reportedID int, reason string) error
	GetAllProfiles() ([]*Profile, error)
	UpdateLastConnection(userID int) error
	SetOnline(userID int, isOnline bool) error
	UpdateFameRating(userID int) error
	BlockUser(blockerID, blockedID int) error
	UnblockUser(blockerID, blockedID int) error
	GetBlockedUsers(userID int) ([]BlockedUser, error)
	CleanupInactiveUsers(timeoutMinutes int) error
	GetUserOnlineStatus(userID int) (bool, *time.Time, error)
	// Méthodes pour les rapports (admin)
	GetAllReports() ([]ReportData, error)
	ProcessReport(reportID int, adminComment, action string) error
}
