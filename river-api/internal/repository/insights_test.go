package repository

import (
	"testing"
	"time"

	"river-api/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newInsightsRepo(t *testing.T) (InsightsRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.WatchProgress{}))
	return NewInsightsRepository(db), db
}

// seedProgress inserts a watch_progress row and forces its updated_at, since
// GORM stamps it to now() on create and the window filters key off updated_at.
func seedProgress(t *testing.T, db *gorm.DB, p models.WatchProgress, updatedAt time.Time) {
	t.Helper()
	require.NoError(t, db.Create(&p).Error)
	require.NoError(t, db.Model(&models.WatchProgress{}).
		Where("id = ?", p.ID).
		UpdateColumn("updated_at", updatedAt).Error)
}

func TestInsightsRepository_WatchTotals(t *testing.T) {
	repo, db := newInsightsRepo(t)
	now := time.Now()

	seedProgress(t, db, models.WatchProgress{UserID: "u1", MediaType: "movie", MediaID: "m1", Position: 100, Duration: 120, Completed: true}, now)
	seedProgress(t, db, models.WatchProgress{UserID: "u1", MediaType: "movie", MediaID: "m2", Position: 50, Duration: 200, Completed: false}, now)
	// Outside a 30d window — should be excluded when a recent bound is applied.
	seedProgress(t, db, models.WatchProgress{UserID: "u2", MediaType: "movie", MediaID: "m3", Position: 999, Duration: 1000, Completed: true}, now.AddDate(0, 0, -60))

	// All-time: everything counts.
	all, err := repo.WatchTotals(time.Time{})
	require.NoError(t, err)
	assert.Equal(t, float64(1149), all.TotalWatchSeconds)
	assert.Equal(t, int64(3), all.Started)
	assert.Equal(t, int64(2), all.Completed)

	// Last 30 days: the 60-day-old row drops out.
	recent, err := repo.WatchTotals(now.AddDate(0, 0, -30))
	require.NoError(t, err)
	assert.Equal(t, float64(150), recent.TotalWatchSeconds)
	assert.Equal(t, int64(2), recent.Started)
	assert.Equal(t, int64(1), recent.Completed)
}

func TestInsightsRepository_WatchTotals_Empty(t *testing.T) {
	repo, _ := newInsightsRepo(t)
	totals, err := repo.WatchTotals(time.Time{})
	require.NoError(t, err)
	assert.Equal(t, WatchTotals{}, totals)
}

func TestInsightsRepository_TopTitles(t *testing.T) {
	repo, db := newInsightsRepo(t)
	now := time.Now()

	// m1 watched by two users → 2 plays, 250s, 1 completion.
	seedProgress(t, db, models.WatchProgress{UserID: "u1", MediaType: "movie", MediaID: "m1", Position: 150, Duration: 160, Completed: true}, now)
	seedProgress(t, db, models.WatchProgress{UserID: "u2", MediaType: "movie", MediaID: "m1", Position: 100, Duration: 160, Completed: false}, now)
	// ep1 watched once → fewer seconds, should rank below m1.
	seedProgress(t, db, models.WatchProgress{UserID: "u1", MediaType: "episode", MediaID: "ep1", Position: 30, Duration: 40, Completed: false}, now)

	rows, err := repo.TopTitles(time.Time{}, 10)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	assert.Equal(t, "m1", rows[0].MediaID)
	assert.Equal(t, "movie", rows[0].MediaType)
	assert.Equal(t, int64(2), rows[0].Plays)
	assert.Equal(t, int64(1), rows[0].Completions)
	assert.Equal(t, float64(250), rows[0].WatchSeconds)

	assert.Equal(t, "ep1", rows[1].MediaID)
	assert.Equal(t, float64(30), rows[1].WatchSeconds)

	// limit is honoured.
	limited, err := repo.TopTitles(time.Time{}, 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
	assert.Equal(t, "m1", limited[0].MediaID)
}

func TestInsightsRepository_PerUserWatch(t *testing.T) {
	repo, db := newInsightsRepo(t)
	now := time.Now()

	seedProgress(t, db, models.WatchProgress{UserID: "u1", MediaType: "movie", MediaID: "m1", Position: 100, Duration: 120, Completed: true}, now)
	seedProgress(t, db, models.WatchProgress{UserID: "u1", MediaType: "movie", MediaID: "m2", Position: 200, Duration: 220, Completed: true}, now)
	seedProgress(t, db, models.WatchProgress{UserID: "u2", MediaType: "movie", MediaID: "m1", Position: 50, Duration: 120, Completed: false}, now)

	rows, err := repo.PerUserWatch(time.Time{})
	require.NoError(t, err)
	require.Len(t, rows, 2)

	// Ordered by watch seconds DESC — u1 leads.
	assert.Equal(t, "u1", rows[0].UserID)
	assert.Equal(t, float64(300), rows[0].WatchSeconds)
	assert.Equal(t, int64(2), rows[0].ItemCount)

	assert.Equal(t, "u2", rows[1].UserID)
	assert.Equal(t, float64(50), rows[1].WatchSeconds)
	assert.Equal(t, int64(1), rows[1].ItemCount)
}

func newLibraryInsightsRepo(t *testing.T) (InsightsRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Library{}, &models.Movie{}, &models.TVShow{}, &models.Season{},
		&models.Episode{}, &models.Track{}, &models.Audiobook{},
		&models.AudiobookChapter{}, &models.FailedJob{},
	))
	return NewInsightsRepository(db), db
}

func TestInsightsRepository_LibraryBreakdown(t *testing.T) {
	repo, db := newLibraryInsightsRepo(t)

	movieLib := models.Library{Base: models.Base{ID: uuid.New()}, Name: "Films", Type: models.LibraryTypeMovie}
	tvLib := models.Library{Base: models.Base{ID: uuid.New()}, Name: "Shows", Type: models.LibraryTypeTVShow}
	musicLib := models.Library{Base: models.Base{ID: uuid.New()}, Name: "Tunes", Type: models.LibraryTypeMusic}
	abLib := models.Library{Base: models.Base{ID: uuid.New()}, Name: "Books", Type: models.LibraryTypeAudiobook}
	for _, l := range []*models.Library{&movieLib, &tvLib, &musicLib, &abLib} {
		require.NoError(t, db.Create(l).Error)
	}

	// Movies: two transcoded, one untranscoded (source but no file).
	require.NoError(t, db.Create(&models.Movie{LibraryID: movieLib.ID, Title: "A", FilePath: "/a.mp4", SizeBytes: 100}).Error)
	require.NoError(t, db.Create(&models.Movie{LibraryID: movieLib.ID, Title: "B", FilePath: "/b.mp4", SizeBytes: 200}).Error)
	require.NoError(t, db.Create(&models.Movie{LibraryID: movieLib.ID, Title: "C", SourcePath: "/c.mkv"}).Error)

	// TV: one show, two sized episodes + one untranscoded.
	show := models.TVShow{Base: models.Base{ID: uuid.New()}, LibraryID: tvLib.ID, Title: "Show"}
	require.NoError(t, db.Create(&show).Error)
	require.NoError(t, db.Create(&models.Episode{TVShowID: show.ID, SeasonID: uuid.New(), Number: 1, FilePath: "/e1.mp4", SizeBytes: 50}).Error)
	require.NoError(t, db.Create(&models.Episode{TVShowID: show.ID, SeasonID: uuid.New(), Number: 2, FilePath: "/e2.mp4", SizeBytes: 70}).Error)
	require.NoError(t, db.Create(&models.Episode{TVShowID: show.ID, SeasonID: uuid.New(), Number: 3, SourcePath: "/e3.mkv"}).Error)

	// Music: two tracks (no source_path → never untranscoded).
	require.NoError(t, db.Create(&models.Track{LibraryID: musicLib.ID, AlbumID: uuid.New(), Title: "T1", FilePath: "/t1.m4a", SizeBytes: 10}).Error)
	require.NoError(t, db.Create(&models.Track{LibraryID: musicLib.ID, AlbumID: uuid.New(), Title: "T2", FilePath: "/t2.m4a", SizeBytes: 20}).Error)

	// Audiobook: one book, size measured over its chapters.
	book := models.Audiobook{Base: models.Base{ID: uuid.New()}, LibraryID: abLib.ID, Title: "Book"}
	require.NoError(t, db.Create(&book).Error)
	require.NoError(t, db.Create(&models.AudiobookChapter{AudiobookID: book.ID, Number: 1, FilePath: "/c1.m4a", SizeBytes: 5}).Error)
	require.NoError(t, db.Create(&models.AudiobookChapter{AudiobookID: book.ID, Number: 2, FilePath: "/c2.m4a", SizeBytes: 15}).Error)

	rows, err := repo.LibraryBreakdown()
	require.NoError(t, err)
	require.Len(t, rows, 4)

	byName := map[string]LibraryHealth{}
	for _, r := range rows {
		byName[r.Name] = r
	}

	assert.Equal(t, int64(3), byName["Films"].ItemCount)
	assert.Equal(t, int64(300), byName["Films"].SizeBytes)
	assert.Equal(t, int64(1), byName["Films"].Untranscoded)

	assert.Equal(t, int64(1), byName["Shows"].ItemCount) // shows, not episodes
	assert.Equal(t, int64(120), byName["Shows"].SizeBytes)
	assert.Equal(t, int64(1), byName["Shows"].Untranscoded)

	assert.Equal(t, int64(2), byName["Tunes"].ItemCount)
	assert.Equal(t, int64(30), byName["Tunes"].SizeBytes)
	assert.Equal(t, int64(0), byName["Tunes"].Untranscoded)

	assert.Equal(t, int64(1), byName["Books"].ItemCount) // audiobooks, not chapters
	assert.Equal(t, int64(20), byName["Books"].SizeBytes)
	assert.Equal(t, int64(0), byName["Books"].Untranscoded)

	// Rows come back ordered by library name.
	assert.Equal(t, []string{"Books", "Films", "Shows", "Tunes"},
		[]string{rows[0].Name, rows[1].Name, rows[2].Name, rows[3].Name})
}

func TestInsightsRepository_FailedJobCount(t *testing.T) {
	repo, db := newLibraryInsightsRepo(t)

	n, err := repo.FailedJobCount()
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)

	require.NoError(t, db.Create(&models.FailedJob{Service: "river-video-trans", Reason: "boom"}).Error)
	require.NoError(t, db.Create(&models.FailedJob{Service: "river-meta-tv", Reason: "429"}).Error)

	n, err = repo.FailedJobCount()
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
}

func TestInsightsRepository_ActivityByDay(t *testing.T) {
	repo, db := newInsightsRepo(t)
	day1 := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	day1later := time.Date(2026, 9, 1, 22, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC)

	seedProgress(t, db, models.WatchProgress{UserID: "u1", MediaType: "movie", MediaID: "m1", Position: 100}, day1)
	seedProgress(t, db, models.WatchProgress{UserID: "u2", MediaType: "movie", MediaID: "m2", Position: 50}, day1later)
	seedProgress(t, db, models.WatchProgress{UserID: "u1", MediaType: "movie", MediaID: "m3", Position: 30}, day2)

	rows, err := repo.ActivityByDay(time.Time{})
	require.NoError(t, err)
	require.Len(t, rows, 2)

	// Ascending by date; same-day rows collapse into one bucket.
	assert.Equal(t, "2026-09-01", rows[0].Date)
	assert.Equal(t, float64(150), rows[0].WatchSeconds)
	assert.Equal(t, int64(2), rows[0].Plays)

	assert.Equal(t, "2026-09-03", rows[1].Date)
	assert.Equal(t, float64(30), rows[1].WatchSeconds)
	assert.Equal(t, int64(1), rows[1].Plays)
}
