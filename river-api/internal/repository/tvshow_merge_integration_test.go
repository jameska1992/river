package repository

import (
	"testing"

	"river-api/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newMergeTestDB spins up an in-memory SQLite DB migrated with just the tables
// the merge transaction touches. The merge SQL is written portably (no DELETE
// aliases), so this exercises the real transaction without needing Postgres.
func newMergeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Library{}, &models.TVShow{}, &models.Season{}, &models.Episode{},
		&models.TVShowPath{}, &models.Person{}, &models.TVShowCast{}, &models.TVShowCrew{},
		&models.WatchlistItem{}, &models.Collection{}, &models.CollectionItem{}, &models.WatchParty{},
	))
	return db
}

func mkEpisode(db *gorm.DB, t *testing.T, seasonID, showID uuid.UUID, num int) {
	t.Helper()
	require.NoError(t, db.Create(&models.Episode{
		SeasonID: seasonID, TVShowID: showID, Number: num, FilePath: "/x.mp4",
	}).Error)
}

func TestMerge_ReparentsRepointsAndPreservesRoot(t *testing.T) {
	db := newMergeTestDB(t)
	repo := NewTVShowMergeRepository(db)

	lib := uuid.New()
	survivor := models.TVShow{LibraryID: lib, Title: "MyShow", FolderPath: "/media/shows/MyShow"}
	merged := models.TVShow{LibraryID: lib, Title: "MyShow", FolderPath: "/truenas/tv/MyShow"}
	require.NoError(t, db.Create(&survivor).Error)
	require.NoError(t, db.Create(&merged).Error)

	// Survivor: Season 1 (ep 1). Merged: Season 1 (ep 2 — no collision, moves
	// into survivor's S1) and Season 2 (whole-season move).
	survS1 := models.Season{TVShowID: survivor.ID, Number: 1}
	mergS1 := models.Season{TVShowID: merged.ID, Number: 1}
	mergS2 := models.Season{TVShowID: merged.ID, Number: 2}
	require.NoError(t, db.Create(&survS1).Error)
	require.NoError(t, db.Create(&mergS1).Error)
	require.NoError(t, db.Create(&mergS2).Error)
	mkEpisode(db, t, survS1.ID, survivor.ID, 1)
	mkEpisode(db, t, mergS1.ID, merged.ID, 2)
	mkEpisode(db, t, mergS2.ID, merged.ID, 1)
	mkEpisode(db, t, mergS2.ID, merged.ID, 2)

	// Watchlist: u1 has BOTH (collision → merged row dropped); u2 has only merged.
	require.NoError(t, db.Create(&models.WatchlistItem{UserID: "u1", MediaType: "tvshow", MediaID: survivor.ID.String()}).Error)
	require.NoError(t, db.Create(&models.WatchlistItem{UserID: "u1", MediaType: "tvshow", MediaID: merged.ID.String()}).Error)
	require.NoError(t, db.Create(&models.WatchlistItem{UserID: "u2", MediaType: "tvshow", MediaID: merged.ID.String()}).Error)

	// Collection c1 has both (collision); c2 has only merged.
	require.NoError(t, db.Create(&models.CollectionItem{CollectionID: "c1", MediaType: "tvshow", MediaID: survivor.ID.String()}).Error)
	require.NoError(t, db.Create(&models.CollectionItem{CollectionID: "c1", MediaType: "tvshow", MediaID: merged.ID.String()}).Error)
	require.NoError(t, db.Create(&models.CollectionItem{CollectionID: "c2", MediaType: "tvshow", MediaID: merged.ID.String()}).Error)

	// Merged carries its own cast/crew (should be dropped) + a watch party.
	person := models.Person{Name: "Actor"}
	require.NoError(t, db.Create(&person).Error)
	require.NoError(t, db.Create(&models.TVShowCast{TVShowID: merged.ID, PersonID: person.ID}).Error)
	require.NoError(t, db.Create(&models.TVShowCrew{TVShowID: merged.ID, PersonID: person.ID, Job: "Director"}).Error)
	require.NoError(t, db.Create(&models.WatchParty{HostID: uuid.New(), MediaType: "episode", MediaID: "e", ShowID: merged.ID.String()}).Error)

	alias := &models.TVShowPath{
		TVShowID: survivor.ID, FolderPath: merged.FolderPath, LibraryID: lib, MergedFromShowID: merged.ID.String(),
	}
	require.NoError(t, repo.Merge(survivor.ID.String(), merged.ID.String(), alias))

	// Seasons/episodes re-parented onto the survivor.
	var epCount int64
	db.Model(&models.Episode{}).Where("tv_show_id = ?", survivor.ID).Count(&epCount)
	assert.Equal(t, int64(4), epCount, "all episodes now belong to the survivor")
	var seasonCount int64
	db.Model(&models.Season{}).Where("tv_show_id = ?", survivor.ID).Count(&seasonCount)
	assert.Equal(t, int64(2), seasonCount, "survivor S1 (with moved ep) + moved S2; merged S1 removed")

	// Merged show is soft-deleted.
	var live models.TVShow
	assert.ErrorIs(t, db.First(&live, "id = ?", merged.ID).Error, gorm.ErrRecordNotFound)

	// Absorbed root preserved and resolvable.
	gotID, err := repo.FindShowIDByPath(lib.String(), "/truenas/tv/MyShow")
	require.NoError(t, err)
	assert.Equal(t, survivor.ID.String(), gotID)

	// Watchlist: both users now point at survivor, no duplicate for u1.
	var wlSurv, wlMerged int64
	db.Model(&models.WatchlistItem{}).Where("media_id = ?", survivor.ID).Count(&wlSurv)
	db.Model(&models.WatchlistItem{}).Where("media_id = ?", merged.ID).Count(&wlMerged)
	assert.Equal(t, int64(2), wlSurv)
	assert.Equal(t, int64(0), wlMerged)

	// Collections: same — collision dropped, the rest repointed.
	var colSurv, colMerged int64
	db.Model(&models.CollectionItem{}).Where("media_id = ?", survivor.ID).Count(&colSurv)
	db.Model(&models.CollectionItem{}).Where("media_id = ?", merged.ID).Count(&colMerged)
	assert.Equal(t, int64(2), colSurv)
	assert.Equal(t, int64(0), colMerged)

	// Merged cast/crew gone; watch party repointed.
	var castCount int64
	db.Model(&models.TVShowCast{}).Where("tv_show_id = ?", merged.ID).Count(&castCount)
	assert.Equal(t, int64(0), castCount)
	var party models.WatchParty
	require.NoError(t, db.First(&party, "media_id = ?", "e").Error)
	assert.Equal(t, survivor.ID.String(), party.ShowID)
}

func TestMerge_FindShowIDByPath_NotFound(t *testing.T) {
	db := newMergeTestDB(t)
	repo := NewTVShowMergeRepository(db)
	_, err := repo.FindShowIDByPath(uuid.New().String(), "/nope")
	assert.Error(t, err)
}

// FindSurvivorID resolves a merged-away show id to its survivor via the alias
// recorded by Merge — the redirect that keeps a late episode write (a transcode
// finishing after a merge) attaching to the surviving show.
func TestMerge_FindSurvivorID(t *testing.T) {
	db := newMergeTestDB(t)
	repo := NewTVShowMergeRepository(db)

	lib := uuid.New()
	survivor := models.TVShow{LibraryID: lib, Title: "MyShow", FolderPath: "/media/shows/MyShow"}
	merged := models.TVShow{LibraryID: lib, Title: "MyShow", FolderPath: "/truenas/tv/MyShow"}
	require.NoError(t, db.Create(&survivor).Error)
	require.NoError(t, db.Create(&merged).Error)

	alias := &models.TVShowPath{
		TVShowID: survivor.ID, FolderPath: merged.FolderPath, LibraryID: lib, MergedFromShowID: merged.ID.String(),
	}
	require.NoError(t, repo.Merge(survivor.ID.String(), merged.ID.String(), alias))

	gotID, err := repo.FindSurvivorID(merged.ID.String())
	require.NoError(t, err)
	assert.Equal(t, survivor.ID.String(), gotID)

	// A show that was never merged has no survivor.
	_, err = repo.FindSurvivorID(uuid.New().String())
	assert.Error(t, err)
}

// FindByIDIncludingDeleted still returns a season after it's soft-deleted,
// unlike FindByID — used to recover a merged-away season's number.
func TestSeason_FindByIDIncludingDeleted(t *testing.T) {
	db := newMergeTestDB(t)
	repo := NewSeasonRepository(db)

	season := models.Season{TVShowID: uuid.New(), Number: 3}
	require.NoError(t, db.Create(&season).Error)
	require.NoError(t, db.Delete(&models.Season{}, "id = ?", season.ID).Error)

	_, err := repo.FindByID(season.ID.String())
	assert.Error(t, err, "soft-deleted season is invisible to FindByID")

	got, err := repo.FindByIDIncludingDeleted(season.ID.String())
	require.NoError(t, err)
	assert.Equal(t, 3, got.Number)

	_, err = repo.FindByIDIncludingDeleted(uuid.New().String())
	assert.Error(t, err)
}
