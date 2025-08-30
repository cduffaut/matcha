// internal/user/profile_repository.go
package user

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/cduffaut/matcha/internal/models"
)

type PostgresProfileRepository struct{ db *sql.DB }

func NewPostgresProfileRepository(db *sql.DB) ProfileRepository {
	return &PostgresProfileRepository{db: db}
}

func (r *PostgresProfileRepository) GetByUserID(userID int) (*Profile, error) {
	const q = `
		SELECT user_id, gender, sexual_preferences, biography, birth_date, fame_rating,
		       latitude, longitude, location_name, created_at, updated_at
		FROM user_profiles
		WHERE user_id = $1`
	p := &Profile{UserID: userID}

	var gender, pref, bio, loc sql.NullString
	var bdate sql.NullTime
	var lat, lon sql.NullFloat64

	err := r.db.QueryRow(q, userID).Scan(
		&p.UserID, &gender, &pref, &bio, &bdate, &p.FameRating,
		&lat, &lon, &loc, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return p, nil
		}
		return nil, fmt.Errorf("get profile: %w", err)
	}

	if gender.Valid {
		p.Gender = Gender(gender.String)
	}
	if pref.Valid {
		p.SexualPreference = SexualPreference(pref.String)
	}
	if bio.Valid {
		p.Biography = bio.String
	}
	if bdate.Valid {
		p.BirthDate = &bdate.Time
	}
	if lat.Valid {
		p.Latitude = lat.Float64
	}
	if lon.Valid {
		p.Longitude = lon.Float64
	}
	if loc.Valid {
		p.LocationName = loc.String
	}

	if tags, err := r.GetTagsByUserID(userID); err == nil {
		p.Tags = tags
	} else {
		return nil, fmt.Errorf("get profile tags: %w", err)
	}
	if photos, err := r.GetPhotosByUserID(userID); err == nil {
		p.Photos = photos
	} else {
		return nil, fmt.Errorf("get profile photos: %w", err)
	}

	return p, nil
}

func (r *PostgresProfileRepository) Create(p *Profile) error {
	const q = `
		INSERT INTO user_profiles (
			user_id, gender, sexual_preferences, biography, birth_date, fame_rating,
			latitude, longitude, location_name
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (user_id) DO NOTHING
		RETURNING created_at, updated_at`
	if err := r.db.QueryRow(
		q, p.UserID, p.Gender, p.SexualPreference, p.Biography, p.BirthDate,
		p.FameRating, p.Latitude, p.Longitude, p.LocationName,
	).Scan(&p.CreatedAt, &p.UpdatedAt); err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("create profile: %w", err)
	}
	return nil
}

func (r *PostgresProfileRepository) Update(p *Profile) error {
	const q = `
		UPDATE user_profiles
		SET gender=$2, sexual_preferences=$3, biography=$4, birth_date=$5,
		    latitude=$6, longitude=$7, location_name=$8, updated_at=CURRENT_TIMESTAMP
		WHERE user_id=$1
		RETURNING updated_at`
	var upd time.Time
	if err := r.db.QueryRow(
		q, p.UserID, p.Gender, p.SexualPreference, p.Biography, p.BirthDate,
		p.Latitude, p.Longitude, p.LocationName,
	).Scan(&upd); err != nil {
		if err == sql.ErrNoRows {
			return r.Create(p)
		}
		return fmt.Errorf("update profile: %w", err)
	}
	p.UpdatedAt = upd
	_ = r.UpdateFameRating(p.UserID)
	return nil
}

func (r *PostgresProfileRepository) AddTag(userID int, tagName string) error {
	var tagID int
	if err := r.db.QueryRow(`SELECT id FROM tags WHERE name=$1`, tagName).Scan(&tagID); err != nil {
		if err != sql.ErrNoRows {
			return fmt.Errorf("check tag: %w", err)
		}
		if err := r.db.QueryRow(`INSERT INTO tags(name) VALUES($1) RETURNING id`, tagName).Scan(&tagID); err != nil {
			return fmt.Errorf("create tag: %w", err)
		}
	}
	if _, err := r.db.Exec(
		`INSERT INTO user_tags(user_id, tag_id) VALUES($1,$2) ON CONFLICT(user_id,tag_id) DO NOTHING`,
		userID, tagID,
	); err != nil {
		return fmt.Errorf("attach tag: %w", err)
	}
	return nil
}

func (r *PostgresProfileRepository) RemoveTag(userID, tagID int) error {
	if _, err := r.db.Exec(`DELETE FROM user_tags WHERE user_id=$1 AND tag_id=$2`, userID, tagID); err != nil {
		return fmt.Errorf("remove tag: %w", err)
	}
	return nil
}

func (r *PostgresProfileRepository) GetTagsByUserID(userID int) ([]Tag, error) {
	const q = `
		SELECT t.id, t.name, t.created_at
		FROM tags t
		JOIN user_tags ut ON t.id=ut.tag_id
		WHERE ut.user_id=$1`
	rows, err := r.db.Query(q, userID)
	if err != nil {
		return nil, fmt.Errorf("get tags: %w", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter tags: %w", err)
	}
	return tags, nil
}

func (r *PostgresProfileRepository) GetAllTags() ([]Tag, error) {
	rows, err := r.db.Query(`SELECT id, name, created_at FROM tags ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("get all tags: %w", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter tags: %w", err)
	}
	return tags, nil
}

func (r *PostgresProfileRepository) GetTagByID(tagID int) (*Tag, error) {
	var t Tag
	if err := r.db.QueryRow(`SELECT id, name, created_at FROM tags WHERE id=$1`, tagID).
		Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tag %d introuvable", tagID)
		}
		return nil, fmt.Errorf("get tag: %w", err)
	}
	return &t, nil
}

func (r *PostgresProfileRepository) AddPhoto(p *Photo) error {
	var cnt int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM user_photos WHERE user_id=$1`, p.UserID).Scan(&cnt); err != nil {
		return fmt.Errorf("count photos: %w", err)
	}
	if cnt >= 5 {
		return fmt.Errorf("limite de 5 photos atteinte")
	}
	if cnt == 0 {
		p.IsProfile = true
	}
	const q = `
		INSERT INTO user_photos(user_id, file_path, is_profile)
		VALUES($1,$2,$3)
		RETURNING id, created_at, updated_at`
	if err := r.db.QueryRow(q, p.UserID, p.FilePath, p.IsProfile).
		Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return fmt.Errorf("add photo: %w", err)
	}
	return nil
}

func (r *PostgresProfileRepository) RemovePhoto(photoID int) error {
	var isProfile bool
	var userID int
	if err := r.db.QueryRow(`SELECT is_profile, user_id FROM user_photos WHERE id=$1`, photoID).
		Scan(&isProfile, &userID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("photo non trouvée")
		}
		return fmt.Errorf("check photo: %w", err)
	}
	if _, err := r.db.Exec(`DELETE FROM user_photos WHERE id=$1`, photoID); err != nil {
		return fmt.Errorf("delete photo: %w", err)
	}
	if isProfile {
		_, _ = r.db.Exec(`
			UPDATE user_photos SET is_profile=true
			WHERE id = (
				SELECT id FROM user_photos WHERE user_id=$1 ORDER BY created_at LIMIT 1
			)`, userID)
	}
	return nil
}

func (r *PostgresProfileRepository) SetProfilePhoto(photoID int) error {
	var userID int
	if err := r.db.QueryRow(`SELECT user_id FROM user_photos WHERE id=$1`, photoID).Scan(&userID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("photo non trouvée")
		}
		return fmt.Errorf("get photo user: %w", err)
	}
	if _, err := r.db.Exec(`UPDATE user_photos SET is_profile=false WHERE user_id=$1`, userID); err != nil {
		return fmt.Errorf("reset profile flags: %w", err)
	}
	if _, err := r.db.Exec(`UPDATE user_photos SET is_profile=true WHERE id=$1`, photoID); err != nil {
		return fmt.Errorf("set profile photo: %w", err)
	}
	return nil
}

func (r *PostgresProfileRepository) IsProfilePhoto(userID, photoID int) (bool, error) {
	var isProfile bool
	if err := r.db.QueryRow(
		`SELECT is_profile FROM user_photos WHERE id=$1 AND user_id=$2`, photoID, userID,
	).Scan(&isProfile); err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("photo non trouvée pour cet utilisateur")
		}
		return false, fmt.Errorf("check profile photo: %w", err)
	}
	return isProfile, nil
}

func (r *PostgresProfileRepository) GetPhotosByUserID(userID int) ([]Photo, error) {
	const q = `
		SELECT id, user_id, file_path, is_profile, created_at, updated_at
		FROM user_photos
		WHERE user_id=$1
		ORDER BY CASE WHEN is_profile THEN 0 ELSE 1 END, created_at`
	rows, err := r.db.Query(q, userID)
	if err != nil {
		return nil, fmt.Errorf("get photos: %w", err)
	}
	defer rows.Close()

	var photos []Photo
	for rows.Next() {
		var p Photo
		if err := rows.Scan(&p.ID, &p.UserID, &p.FilePath, &p.IsProfile, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan photo: %w", err)
		}
		photos = append(photos, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter photos: %w", err)
	}
	return photos, nil
}

func (r *PostgresProfileRepository) RecordVisit(visitorID, visitedID int) error {
	if visitorID == visitedID {
		return nil
	}
	const q = `
		INSERT INTO profile_visits(visitor_id, visited_id)
		VALUES($1,$2)
		ON CONFLICT(visitor_id, visited_id) DO UPDATE SET visited_at=CURRENT_TIMESTAMP`
	if _, err := r.db.Exec(q, visitorID, visitedID); err != nil {
		return fmt.Errorf("record visit: %w", err)
	}
	return r.UpdateFameRating(visitedID)
}

func (r *PostgresProfileRepository) GetVisitorsForUser(userID int) ([]ProfileVisit, error) {
	const q = `
		SELECT pv.id, pv.visitor_id, pv.visited_id, pv.visited_at,
		       u.username, u.first_name, u.last_name
		FROM profile_visits pv
		JOIN users u ON pv.visitor_id = u.id
		WHERE pv.visited_id=$1
		ORDER BY pv.visited_at DESC`
	rows, err := r.db.Query(q, userID)
	if err != nil {
		return nil, fmt.Errorf("get visitors: %w", err)
	}
	defer rows.Close()

	var out []ProfileVisit
	for rows.Next() {
		var v ProfileVisit
		var un, fn, ln string
		if err := rows.Scan(&v.ID, &v.VisitorID, &v.VisitedID, &v.VisitedAt, &un, &fn, &ln); err != nil {
			return nil, fmt.Errorf("scan visit: %w", err)
		}
		v.Visitor = &models.User{ID: v.VisitorID, Username: un, FirstName: fn, LastName: ln}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter visitors: %w", err)
	}
	return out, nil
}

func (r *PostgresProfileRepository) LikeUser(likerID, likedID int) error {
	var has bool
	if err := r.db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM user_photos WHERE user_id=$1 AND is_profile=true)`,
		likerID,
	).Scan(&has); err != nil {
		return fmt.Errorf("check profile photo: %w", err)
	}
	if !has {
		return fmt.Errorf("vous devez avoir une photo de profil pour liker un utilisateur")
	}
	if _, err := r.db.Exec(
		`INSERT INTO user_likes(liker_id, liked_id) VALUES($1,$2) ON CONFLICT DO NOTHING`,
		likerID, likedID,
	); err != nil {
		return fmt.Errorf("like user: %w", err)
	}
	return r.UpdateFameRating(likedID)
}

func (r *PostgresProfileRepository) UnlikeUser(likerID, likedID int) error {
	if _, err := r.db.Exec(`DELETE FROM user_likes WHERE liker_id=$1 AND liked_id=$2`, likerID, likedID); err != nil {
		return fmt.Errorf("unlike user: %w", err)
	}
	return r.UpdateFameRating(likedID)
}

func (r *PostgresProfileRepository) GetLikesForUser(userID int) ([]UserLike, error) {
	const q = `
		SELECT ul.id, ul.liker_id, ul.liked_id, ul.created_at,
		       u.username, u.first_name, u.last_name
		FROM user_likes ul
		JOIN users u ON ul.liker_id = u.id
		WHERE ul.liked_id=$1
		ORDER BY ul.created_at DESC`
	rows, err := r.db.Query(q, userID)
	if err != nil {
		return nil, fmt.Errorf("get likes: %w", err)
	}
	defer rows.Close()

	var out []UserLike
	for rows.Next() {
		var l UserLike
		var un, fn, ln string
		if err := rows.Scan(&l.ID, &l.LikerID, &l.LikedID, &l.CreatedAt, &un, &fn, &ln); err != nil {
			return nil, fmt.Errorf("scan like: %w", err)
		}
		l.Liker = &models.User{ID: l.LikerID, Username: un, FirstName: fn, LastName: ln}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter likes: %w", err)
	}
	return out, nil
}

func (r *PostgresProfileRepository) CheckIfLiked(likerID, likedID int) (bool, error) {
	var ok bool
	if err := r.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM user_likes WHERE liker_id=$1 AND liked_id=$2)`,
		likerID, likedID,
	).Scan(&ok); err != nil {
		return false, fmt.Errorf("check liked: %w", err)
	}
	return ok, nil
}

func (r *PostgresProfileRepository) GetAllProfiles() ([]*Profile, error) {
	rows, err := r.db.Query(`
		SELECT user_id, gender, sexual_preferences, biography, birth_date, fame_rating,
		       latitude, longitude, location_name, created_at, updated_at
		FROM user_profiles`)
	if err != nil {
		return nil, fmt.Errorf("get all profiles: %w", err)
	}
	defer rows.Close()

	var out []*Profile
	for rows.Next() {
		var p Profile
		var gender, pref, bio, loc sql.NullString
		var bdate sql.NullTime
		var lat, lon sql.NullFloat64

		if err := rows.Scan(
			&p.UserID, &gender, &pref, &bio, &bdate, &p.FameRating,
			&lat, &lon, &loc, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan profile: %w", err)
		}
		if gender.Valid {
			p.Gender = Gender(gender.String)
		}
		if pref.Valid {
			p.SexualPreference = SexualPreference(pref.String)
		}
		if bio.Valid {
			p.Biography = bio.String
		}
		if bdate.Valid {
			p.BirthDate = &bdate.Time
		}
		if lat.Valid {
			p.Latitude = lat.Float64
		}
		if lon.Valid {
			p.Longitude = lon.Float64
		}
		if loc.Valid {
			p.LocationName = loc.String
		}
		if tags, err := r.GetTagsByUserID(p.UserID); err == nil {
			p.Tags = tags
		}
		if photos, err := r.GetPhotosByUserID(p.UserID); err == nil {
			p.Photos = photos
		}
		out = append(out, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter profiles: %w", err)
	}
	return out, nil
}

func (r *PostgresProfileRepository) IsBlocked(userID1, userID2 int) (bool, error) {
	var blocked bool
	if err := r.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM user_blocks
			WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1)
		)`, userID1, userID2,
	).Scan(&blocked); err != nil {
		return false, fmt.Errorf("check blocked: %w", err)
	}
	return blocked, nil
}

func (r *PostgresProfileRepository) CheckIfMatched(user1ID, user2ID int) (bool, error) {
	var matched bool
	if err := r.db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM user_likes WHERE liker_id=$1 AND liked_id=$2)
		   AND EXISTS(SELECT 1 FROM user_likes WHERE liker_id=$2 AND liked_id=$1)`,
		user1ID, user2ID,
	).Scan(&matched); err != nil {
		return false, fmt.Errorf("check matched: %w", err)
	}
	return matched, nil
}

func (r *PostgresProfileRepository) UpdateFameRating(userID int) error {
	const q = `
		WITH user_stats AS (
			SELECT 
				COALESCE((SELECT COUNT(*) FROM profile_visits WHERE visited_id=$1),0) AS visits,
				COALESCE((SELECT COUNT(*) FROM user_likes WHERE liked_id=$1),0) AS likes,
				COALESCE((
					SELECT COUNT(*) FROM user_likes ul1
					WHERE ul1.liked_id=$1
					  AND EXISTS (
						SELECT 1 FROM user_likes ul2
						WHERE ul2.liker_id=ul1.liked_id AND ul2.liked_id=ul1.liker_id
					  )
				),0) AS matches
		)
		UPDATE user_profiles
		SET fame_rating = (SELECT LEAST(100, visits + likes*2 + matches*5) FROM user_stats)
		WHERE user_id=$1`
	if _, err := r.db.Exec(q, userID); err != nil {
		return fmt.Errorf("update fame: %w", err)
	}
	return nil
}

func (r *PostgresProfileRepository) UnblockUser(blockerID, blockedID int) error {
	if _, err := r.db.Exec(`DELETE FROM user_blocks WHERE blocker_id=$1 AND blocked_id=$2`, blockerID, blockedID); err != nil {
		return fmt.Errorf("unblock user: %w", err)
	}
	return nil
}

func (r *PostgresProfileRepository) GetBlockedUsers(userID int) ([]BlockedUser, error) {
	const q = `
		SELECT ub.id, ub.blocker_id, ub.blocked_id, ub.created_at,
		       u.username, u.first_name, u.last_name
		FROM user_blocks ub
		JOIN users u ON ub.blocked_id = u.id
		WHERE ub.blocker_id=$1
		ORDER BY ub.created_at DESC`
	rows, err := r.db.Query(q, userID)
	if err != nil {
		return nil, fmt.Errorf("get blocked users: %w", err)
	}
	defer rows.Close()

	var out []BlockedUser
	for rows.Next() {
		var b BlockedUser
		var un, fn, ln string
		if err := rows.Scan(&b.ID, &b.BlockerID, &b.BlockedID, &b.CreatedAt, &un, &fn, &ln); err != nil {
			return nil, fmt.Errorf("scan blocked: %w", err)
		}
		b.User = &models.User{ID: b.BlockedID, Username: un, FirstName: fn, LastName: ln}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter blocked: %w", err)
	}
	return out, nil
}

func (r *PostgresProfileRepository) BlockUser(blockerID, blockedID int) error {
	if _, err := r.db.Exec(
		`INSERT INTO user_blocks(blocker_id, blocked_id) VALUES($1,$2) ON CONFLICT DO NOTHING`,
		blockerID, blockedID,
	); err != nil {
		return fmt.Errorf("block user: %w", err)
	}
	return nil
}

func (r *PostgresProfileRepository) ReportUser(reporterID, reportedID int, reason string) error {
	if _, err := r.db.Exec(`
		INSERT INTO user_reports(reporter_id, reported_id, reason)
		VALUES($1,$2,$3)
		ON CONFLICT(reporter_id, reported_id) DO UPDATE
		SET reason=$3, created_at=CURRENT_TIMESTAMP`,
		reporterID, reportedID, reason,
	); err != nil {
		return fmt.Errorf("report user: %w", err)
	}
	return nil
}

func (r *PostgresProfileRepository) GetAllReports() ([]ReportData, error) {
	const q = `
		SELECT ur.id, ur.reporter_id, ur.reported_id, ur.reason, ur.created_at,
		       ur.is_processed, ur.processed_at, ur.admin_comment,
		       u1.username, u1.first_name, u1.last_name,
		       u2.username, u2.first_name, u2.last_name
		FROM user_reports ur
		JOIN users u1 ON ur.reporter_id = u1.id
		JOIN users u2 ON ur.reported_id = u2.id
		ORDER BY ur.created_at DESC`
	rows, err := r.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("get reports: %w", err)
	}
	defer rows.Close()

	var out []ReportData
	for rows.Next() {
		var rd ReportData
		var pAt sql.NullTime
		var admin sql.NullString
		var repU, repFN, repLN string
		var redU, redFN, redLN string

		if err := rows.Scan(
			&rd.ID, &rd.ReporterID, &rd.ReportedID, &rd.Reason, &rd.CreatedAt,
			&rd.IsProcessed, &pAt, &admin,
			&repU, &repFN, &repLN, &redU, &redFN, &redLN,
		); err != nil {
			return nil, fmt.Errorf("scan report: %w", err)
		}
		if pAt.Valid {
			rd.ProcessedAt = &pAt.Time
		}
		if admin.Valid {
			rd.AdminComment = admin.String
		}
		rd.ReporterInfo = &models.User{ID: rd.ReporterID, Username: repU, FirstName: repFN, LastName: repLN}
		rd.ReportedInfo = &models.User{ID: rd.ReportedID, Username: redU, FirstName: redFN, LastName: redLN}
		out = append(out, rd)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter reports: %w", err)
	}
	return out, nil
}

func (r *PostgresProfileRepository) GetUserOnlineStatus(userID int) (bool, *time.Time, error) {
	var online bool
	var last sql.NullTime
	if err := r.db.QueryRow(
		`SELECT is_online, last_connection FROM user_profiles WHERE user_id=$1`, userID,
	).Scan(&online, &last); err != nil {
		if err == sql.ErrNoRows {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf("get status: %w", err)
	}
	var t *time.Time
	if last.Valid {
		t = &last.Time
	}
	return online, t, nil
}

func (r *PostgresProfileRepository) SetOnline(userID int, isOnline bool) error {
	nowUTC := time.Now().UTC()
	if _, err := r.db.Exec(`
		UPDATE user_profiles
		SET is_online=$2, last_connection=$3
		WHERE user_id=$1`, userID, isOnline, nowUTC,
	); err != nil {
		return fmt.Errorf("set online: %w", err)
	}
	return nil
}

func (r *PostgresProfileRepository) CleanupInactiveUsers(timeoutMinutes int) error {
	cutoff := time.Now().UTC().Add(-time.Duration(timeoutMinutes) * time.Minute)
	if _, err := r.db.Exec(`UPDATE user_profiles SET is_online=false WHERE is_online=true AND last_connection < $1`, cutoff); err != nil {
		return fmt.Errorf("cleanup inactive: %w", err)
	}
	return nil
}

func (r *PostgresProfileRepository) ProcessReport(reportID int, adminComment, action string) error {
	if _, err := r.db.Exec(`
		UPDATE user_reports
		SET is_processed=true, processed_at=CURRENT_TIMESTAMP, admin_comment=$2
		WHERE id=$1`, reportID, adminComment,
	); err != nil {
		return fmt.Errorf("process report: %w", err)
	}
	return nil
}

func (r *PostgresProfileRepository) UpdateLastConnection(userID int) error {
	if _, err := r.db.Exec(`
		UPDATE user_profiles SET last_connection=CURRENT_TIMESTAMP WHERE user_id=$1`, userID,
	); err != nil {
		return fmt.Errorf("update last connection: %w", err)
	}
	return nil
}
