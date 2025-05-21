package user

import (
	"database/sql"
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
	Gender           Gender           `db:"gender"`
	SexualPreference SexualPreference `db:"sexual_preference"`
	Biography        string           `db:"biography"`
	BirthDate        *time.Time       `db:"birth_date"`
	Latitude         float64          `db:"latitude"`
	Longitude        float64          `db:"longitude"`
	LocationName     string           `db:"location_name"`
	LastConnection   sql.NullTime     `db:"last_connection"`
	IsOnline         bool             `db:"is_online"`
	FameRating       int              `db:"fame_rating"`
	Tags             []Tag            `db:"-"`
	Photos           []Photo          `db:"-"`
	CreatedAt        time.Time        `db:"created_at"`
	UpdatedAt        time.Time        `db:"updated_at"`
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
	BlockUser(blockerID, blockedID int) error
	IsBlocked(userID, blockedID int) (bool, error)
	ReportUser(reporterID, reportedID int, reason string) error
	GetAllProfiles() ([]*Profile, error)
	UpdateLastConnection(userID int) error
	SetOnline(userID int, isOnline bool) error
	UpdateFameRating(userID int) error
}
