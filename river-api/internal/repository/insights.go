package repository

import (
	"sort"
	"time"

	"river-api/internal/models"

	"gorm.io/gorm"
)

// InsightsRepository aggregates watch_progress rows for the admin Insights
// dashboard. Everything here is a read-only rollup — GROUP BY / SUM over the
// existing progress table, no new writes.
//
// Note on the data model: WatchProgress holds one row per (user, media_type,
// media_id), upserted in place. So "watch seconds" is the furthest position
// reached (not cumulative playback across rewatches) and time-bucketing keys
// off updated_at (the last touch), not a per-view event log. These are
// deliberate approximations given the available data.
type InsightsRepository interface {
	// WatchTotals returns the aggregate watch seconds plus started/completed
	// counts for rows touched since the given time.
	WatchTotals(since time.Time) (WatchTotals, error)
	// TopTitles returns the most-watched (media_type, media_id) pairs by watch
	// seconds, capped at limit. Titles are resolved by the service layer.
	TopTitles(since time.Time, limit int) ([]TopTitle, error)
	// PerUserWatch returns watch seconds + item count grouped by user.
	PerUserWatch(since time.Time) ([]UserWatch, error)
	// ActivityByDay buckets watch seconds + play counts by calendar day (UTC),
	// ascending. Bucketing is done in Go so the query stays portable across
	// Postgres (prod) and SQLite (tests) — no date_trunc/strftime dialect split.
	ActivityByDay(since time.Time) ([]DayBucket, error)
	// LibraryBreakdown returns per-library item counts, on-disk size, and the
	// untranscoded tally, one row per library (ordered by name).
	LibraryBreakdown() ([]LibraryHealth, error)
	// FailedJobCount returns the total number of dead-lettered ingest jobs.
	FailedJobCount() (int64, error)
}

// LibraryHealth is the storage/health rollup for a single library.
//
// ItemCount counts the library's primary browsable entity (movies, shows,
// tracks, audiobooks) to match the admin Overview. SizeBytes and Untranscoded
// are measured over the underlying media files (episodes for a show library,
// chapters for an audiobook library). Untranscoded — source discovered but no
// output yet (source_path set, file_path empty) — only applies to video
// libraries; music/audiobook rows carry no source_path, so it's always 0 there.
type LibraryHealth struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	ItemCount    int64  `json:"item_count"`
	SizeBytes    int64  `json:"size_bytes"`
	Untranscoded int64  `json:"untranscoded"`
}

// WatchTotals is the headline rollup: total seconds watched, how many progress
// rows exist (started), and how many are completed.
type WatchTotals struct {
	TotalWatchSeconds float64
	Started           int64
	Completed         int64
}

// TopTitle is one (media_type, media_id) aggregate before title resolution.
type TopTitle struct {
	MediaType    string
	MediaID      string
	Plays        int64
	Completions  int64
	WatchSeconds float64
}

// UserWatch is one per-user aggregate before username resolution.
type UserWatch struct {
	UserID       string
	WatchSeconds float64
	ItemCount    int64
}

// DayBucket is watch activity for a single calendar day (YYYY-MM-DD, UTC).
type DayBucket struct {
	Date         string
	WatchSeconds float64
	Plays        int64
}

type gormInsightsRepository struct{ db *gorm.DB }

func NewInsightsRepository(db *gorm.DB) InsightsRepository {
	return &gormInsightsRepository{db: db}
}

func (r *gormInsightsRepository) WatchTotals(since time.Time) (WatchTotals, error) {
	var out WatchTotals
	// CASE WHEN (not SUM(completed)) so the boolean sum is portable: Postgres
	// refuses SUM() over a bool column, SQLite would allow it.
	err := r.db.Model(&models.WatchProgress{}).
		Select("COALESCE(SUM(position), 0) AS total_watch_seconds, "+
			"COUNT(*) AS started, "+
			"COALESCE(SUM(CASE WHEN completed THEN 1 ELSE 0 END), 0) AS completed").
		Where("updated_at > ?", since).
		Scan(&out).Error
	return out, err
}

func (r *gormInsightsRepository) TopTitles(since time.Time, limit int) ([]TopTitle, error) {
	var rows []TopTitle
	err := r.db.Model(&models.WatchProgress{}).
		Select("media_type, media_id, "+
			"COUNT(*) AS plays, "+
			"COALESCE(SUM(CASE WHEN completed THEN 1 ELSE 0 END), 0) AS completions, "+
			"COALESCE(SUM(position), 0) AS watch_seconds").
		Where("updated_at > ?", since).
		Group("media_type, media_id").
		Order("watch_seconds DESC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

func (r *gormInsightsRepository) PerUserWatch(since time.Time) ([]UserWatch, error) {
	var rows []UserWatch
	err := r.db.Model(&models.WatchProgress{}).
		Select("user_id, "+
			"COALESCE(SUM(position), 0) AS watch_seconds, "+
			"COUNT(*) AS item_count").
		Where("updated_at > ?", since).
		Group("user_id").
		Order("watch_seconds DESC").
		Scan(&rows).Error
	return rows, err
}

func (r *gormInsightsRepository) ActivityByDay(since time.Time) ([]DayBucket, error) {
	var rows []struct {
		UpdatedAt time.Time
		Position  float64
	}
	if err := r.db.Model(&models.WatchProgress{}).
		Select("updated_at, position").
		Where("updated_at > ?", since).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	byDay := make(map[string]*DayBucket)
	for _, row := range rows {
		day := row.UpdatedAt.UTC().Format("2006-01-02")
		b, ok := byDay[day]
		if !ok {
			b = &DayBucket{Date: day}
			byDay[day] = b
		}
		b.WatchSeconds += row.Position
		b.Plays++
	}

	out := make([]DayBucket, 0, len(byDay))
	for _, b := range byDay {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}

func (r *gormInsightsRepository) LibraryBreakdown() ([]LibraryHealth, error) {
	var libs []models.Library
	if err := r.db.Order("name").Find(&libs).Error; err != nil {
		return nil, err
	}
	out := make([]LibraryHealth, 0, len(libs))
	for _, lib := range libs {
		id := lib.ID.String()
		h := LibraryHealth{ID: id, Name: lib.Name, Type: string(lib.Type)}
		var err error
		switch lib.Type {
		case models.LibraryTypeMovie:
			if err = r.count(&models.Movie{}, "library_id = ?", []any{id}, &h.ItemCount); err == nil {
				if err = r.sumSize(&models.Movie{}, "library_id = ?", []any{id}, &h.SizeBytes); err == nil {
					err = r.count(&models.Movie{}, "library_id = ? AND source_path <> '' AND file_path = ''", []any{id}, &h.Untranscoded)
				}
			}
		case models.LibraryTypeTVShow:
			// Items = shows (matches Overview); size + untranscoded measured
			// over the episode files, joined back to the library via the show.
			if err = r.count(&models.TVShow{}, "library_id = ?", []any{id}, &h.ItemCount); err == nil {
				if err = r.db.Model(&models.Episode{}).
					Joins("JOIN tv_shows ON tv_shows.id = episodes.tv_show_id").
					Where("tv_shows.library_id = ?", id).
					Select("COALESCE(SUM(episodes.size_bytes), 0)").Scan(&h.SizeBytes).Error; err == nil {
					err = r.db.Model(&models.Episode{}).
						Joins("JOIN tv_shows ON tv_shows.id = episodes.tv_show_id").
						Where("tv_shows.library_id = ? AND episodes.source_path <> '' AND episodes.file_path = ''", id).
						Count(&h.Untranscoded).Error
				}
			}
		case models.LibraryTypeMusic:
			// Tracks carry no source_path → Untranscoded stays 0.
			if err = r.count(&models.Track{}, "library_id = ?", []any{id}, &h.ItemCount); err == nil {
				err = r.sumSize(&models.Track{}, "library_id = ?", []any{id}, &h.SizeBytes)
			}
		case models.LibraryTypeAudiobook:
			// Items = audiobooks (matches Overview); size measured over the
			// chapter files joined back via the audiobook. No source_path.
			if err = r.count(&models.Audiobook{}, "library_id = ?", []any{id}, &h.ItemCount); err == nil {
				err = r.db.Model(&models.AudiobookChapter{}).
					Joins("JOIN audiobooks ON audiobooks.id = audiobook_chapters.audiobook_id").
					Where("audiobooks.library_id = ?", id).
					Select("COALESCE(SUM(audiobook_chapters.size_bytes), 0)").Scan(&h.SizeBytes).Error
			}
		}
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, nil
}

// count runs COUNT(*) over model with the given where clause.
func (r *gormInsightsRepository) count(model any, where string, args []any, dst *int64) error {
	return r.db.Model(model).Where(where, args...).Count(dst).Error
}

// sumSize runs COALESCE(SUM(size_bytes), 0) over model with the given where.
func (r *gormInsightsRepository) sumSize(model any, where string, args []any, dst *int64) error {
	return r.db.Model(model).Where(where, args...).
		Select("COALESCE(SUM(size_bytes), 0)").Scan(dst).Error
}

func (r *gormInsightsRepository) FailedJobCount() (int64, error) {
	var n int64
	return n, r.db.Model(&models.FailedJob{}).Count(&n).Error
}
