package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"river-api/internal/apperrors"
	"river-api/internal/models"
	"river-api/internal/repository"
	"river-api/internal/subdl"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSubdl struct {
	lastParams subdl.SearchParams
	lastURL    string
	subs       []subdl.Subtitle
	searchErr  error
	dl         []byte
	dlErr      error
}

func (f *fakeSubdl) Search(_ context.Context, p subdl.SearchParams) ([]subdl.Subtitle, error) {
	f.lastParams = p
	return f.subs, f.searchErr
}
func (f *fakeSubdl) Download(_ context.Context, u string) ([]byte, error) {
	f.lastURL = u
	return f.dl, f.dlErr
}

type fakeSubtitleRepo struct{ created []repository.SubtitleInput }

func (r *fakeSubtitleRepo) Create(in repository.SubtitleInput) (*models.Subtitle, error) {
	r.created = append(r.created, in)
	return &models.Subtitle{
		Base: models.Base{ID: uuid.New()}, MediaType: in.MediaType, MediaID: in.MediaID,
		Language: in.Language, Label: in.Label, FilePath: in.FilePath,
	}, nil
}
func (r *fakeSubtitleRepo) ListByMedia(string, string) ([]models.Subtitle, error) { return nil, nil }
func (r *fakeSubtitleRepo) FindByID(string) (*models.Subtitle, error) {
	return nil, apperrors.ErrNotFound
}
func (r *fakeSubtitleRepo) Delete(string) error { return nil }

func settingsWithSubDL(key string) *SettingsService {
	m := map[string]string{}
	if key != "" {
		m[keySubDLKey] = key
	}
	return NewSettingsService(&memSettingRepo{m: m})
}

func TestSubtitleSearch_SearchMovie(t *testing.T) {
	movieID := uuid.New()
	movies := &memMovieRepo{movies: []*models.Movie{{Base: models.Base{ID: movieID}, Title: "M", TMDBID: 27205}}}
	f := &fakeSubdl{subs: []subdl.Subtitle{{URL: "/s/1.zip", Language: "English"}}}
	svc := NewSubtitleSearchService(f, settingsWithSubDL("key"), movies, &memEpisodeRepo{}, &memSeasonRepo{}, &memShowRepo{}, &fakeSubtitleRepo{}, t.TempDir())

	subs, err := svc.SearchMovie(context.Background(), movieID.String(), "en,fr")
	require.NoError(t, err)
	require.Len(t, subs, 1)
	assert.Equal(t, 27205, f.lastParams.TmdbID)
	assert.Equal(t, "movie", f.lastParams.Type)
	assert.Equal(t, "EN,FR", f.lastParams.Languages, "languages normalised to upper-case codes")
}

func TestSubtitleSearch_SearchMovie_NoTMDBID(t *testing.T) {
	movieID := uuid.New()
	movies := &memMovieRepo{movies: []*models.Movie{{Base: models.Base{ID: movieID}, Title: "M"}}} // TMDBID 0
	svc := NewSubtitleSearchService(&fakeSubdl{}, settingsWithSubDL("key"), movies, &memEpisodeRepo{}, &memSeasonRepo{}, &memShowRepo{}, &fakeSubtitleRepo{}, t.TempDir())

	_, err := svc.SearchMovie(context.Background(), movieID.String(), "en")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestSubtitleSearch_SearchMovie_NoAPIKey(t *testing.T) {
	movieID := uuid.New()
	movies := &memMovieRepo{movies: []*models.Movie{{Base: models.Base{ID: movieID}, Title: "M", TMDBID: 1}}}
	svc := NewSubtitleSearchService(&fakeSubdl{}, settingsWithSubDL(""), movies, &memEpisodeRepo{}, &memSeasonRepo{}, &memShowRepo{}, &fakeSubtitleRepo{}, t.TempDir())

	_, err := svc.SearchMovie(context.Background(), movieID.String(), "en")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestSubtitleSearch_SearchEpisode(t *testing.T) {
	showID, seasonID, epID := uuid.New(), uuid.New(), uuid.New()
	shows := &memShowRepo{shows: []*models.TVShow{{Base: models.Base{ID: showID}, TMDBID: 1399}}}
	episodes := &memEpisodeRepo{episodes: []*models.Episode{{Base: models.Base{ID: epID}, TVShowID: showID, SeasonID: seasonID, Number: 5}}}
	seasons := &memSeasonRepo{seasons: []*models.Season{{Base: models.Base{ID: seasonID}, TVShowID: showID, Number: 2}}}
	f := &fakeSubdl{subs: []subdl.Subtitle{{URL: "/s/2.zip"}}}
	svc := NewSubtitleSearchService(f, settingsWithSubDL("key"), &memMovieRepo{}, episodes, seasons, shows, &fakeSubtitleRepo{}, t.TempDir())

	_, err := svc.SearchEpisode(context.Background(), epID.String(), "en")
	require.NoError(t, err)
	assert.Equal(t, "tv", f.lastParams.Type)
	assert.Equal(t, 1399, f.lastParams.TmdbID)
	assert.Equal(t, 2, f.lastParams.SeasonNumber)
	assert.Equal(t, 5, f.lastParams.EpisodeNumber)
}

func TestSubtitleSearch_AttachMovie_WritesFileAndRecord(t *testing.T) {
	dir := t.TempDir()
	movieID := uuid.New()
	movies := &memMovieRepo{movies: []*models.Movie{{Base: models.Base{ID: movieID}, Title: "M", TMDBID: 1}}}
	f := &fakeSubdl{dl: []byte("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHi\n")}
	subsRepo := &fakeSubtitleRepo{}
	svc := NewSubtitleSearchService(f, settingsWithSubDL("key"), movies, &memEpisodeRepo{}, &memSeasonRepo{}, &memShowRepo{}, subsRepo, dir)

	sub, err := svc.AttachMovie(context.Background(), movieID.String(), SubtitleAttachInput{URL: "/s/1.zip", Language: "EN"})
	require.NoError(t, err)

	assert.Equal(t, "/s/1.zip", f.lastURL)
	assert.Equal(t, "movie", sub.MediaType)
	assert.Equal(t, movieID.String(), sub.MediaID)
	assert.Equal(t, "en", sub.Language, "language lower-cased")
	assert.Equal(t, "EN", sub.Label, "label defaults to the language code")
	require.Len(t, subsRepo.created, 1)

	assert.True(t, strings.HasPrefix(sub.FilePath, filepath.Join(dir, "subtitles")), "written under MEDIA_BASE_PATH/subtitles")
	content, err := os.ReadFile(sub.FilePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "WEBVTT")
}

func TestSubtitleSearch_AttachMovie_RequiresURL(t *testing.T) {
	movieID := uuid.New()
	movies := &memMovieRepo{movies: []*models.Movie{{Base: models.Base{ID: movieID}, Title: "M", TMDBID: 1}}}
	svc := NewSubtitleSearchService(&fakeSubdl{}, settingsWithSubDL("key"), movies, &memEpisodeRepo{}, &memSeasonRepo{}, &memShowRepo{}, &fakeSubtitleRepo{}, t.TempDir())

	_, err := svc.AttachMovie(context.Background(), movieID.String(), SubtitleAttachInput{URL: ""})
	assert.ErrorIs(t, err, ErrInvalidInput)
}
