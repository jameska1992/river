package services

import (
	"testing"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the thin read / get / count / targeted-update
// passthroughs on the media services — the methods that delegate straight
// to a repository and (for the *Path updaters) touch a single field.

// --- Movies ---

func TestMovieService_ReadPassthroughs(t *testing.T) {
	m := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Metropolis", Genres: `["sci-fi"]`}
	repo := &memMovieRepo{movies: []*models.Movie{m}}
	svc := NewMovieService(repo, &memCleanupRepo{})

	list, err := svc.List(MovieFilter{})
	require.NoError(t, err)
	assert.Len(t, list, 1)

	got, err := svc.GetByID(m.ID.String())
	require.NoError(t, err)
	assert.Equal(t, "Metropolis", got.Title)

	_, err = svc.GetByID(uuid.NewString())
	assert.Error(t, err, "unknown id should error")

	// FindRecent/FindUnidentified/Count are stubbed to zero values in the
	// fake — we're covering the service delegation, not repo behaviour.
	_, err = svc.ListRecent(5)
	require.NoError(t, err)
	_, err = svc.ListUnidentified()
	require.NoError(t, err)
	_, err = svc.Count("")
	require.NoError(t, err)
}

func TestMovieService_UpdateSourceAndFilePath(t *testing.T) {
	m := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Keep", Genres: `["x"]`}
	repo := &memMovieRepo{movies: []*models.Movie{m}}
	svc := NewMovieService(repo, &memCleanupRepo{})

	got, err := svc.UpdateSourcePath(m.ID.String(), "/src/keep.mkv")
	require.NoError(t, err)
	assert.Equal(t, "/src/keep.mkv", got.SourcePath)
	assert.Equal(t, "Keep", got.Title, "title must be untouched")

	got, err = svc.UpdateFilePath(m.ID.String(), "/out/keep.mp4")
	require.NoError(t, err)
	assert.Equal(t, "/out/keep.mp4", got.FilePath)

	_, err = svc.UpdateSourcePath(uuid.NewString(), "/x")
	assert.Error(t, err)
	_, err = svc.UpdateFilePath(uuid.NewString(), "/x")
	assert.Error(t, err)
}

func TestMovieService_Similar_RanksByGenre(t *testing.T) {
	src := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Src", Genres: `["Action","Sci-Fi"]`}
	match := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Match", Genres: `["action"]`}
	miss := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Miss", Genres: `["romance"]`}
	repo := &memMovieRepo{movies: []*models.Movie{src, match, miss}}
	svc := NewMovieService(repo, &memCleanupRepo{})

	got, err := svc.Similar(src.ID.String(), 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Match", got[0].Title)
	assert.Equal(t, "movie", got[0].Type)
}

// --- Libraries ---

func TestLibraryService_ListAndDelete(t *testing.T) {
	lib := &models.Library{Base: models.Base{ID: uuid.New()}, Name: "Films", Type: "movie", Paths: "[]"}
	repo := &memLibraryRepo{libs: []*models.Library{lib}}
	svc := NewLibraryService(repo)

	list, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, list, 1)

	require.NoError(t, svc.Delete(lib.ID.String()))
	list, err = svc.List()
	require.NoError(t, err)
	assert.Empty(t, list)

	assert.Error(t, svc.Delete(uuid.NewString()), "deleting a missing library errors")
}

// --- Music ---

func TestMusicService_ReadPassthroughs(t *testing.T) {
	artist := &models.Artist{Base: models.Base{ID: uuid.New()}, Name: "Bowie"}
	album := &models.Album{Base: models.Base{ID: uuid.New()}, ArtistID: artist.ID, Title: "Low"}
	track := &models.Track{Base: models.Base{ID: uuid.New()}, AlbumID: album.ID, Title: "Speed of Life"}
	svc := NewMusicService(
		&memArtistRepo{artists: []*models.Artist{artist}},
		&memAlbumRepo{albums: []*models.Album{album}},
		&memTrackRepo{tracks: []*models.Track{track}},
	)

	_, err := svc.ListArtists(ArtistFilter{})
	require.NoError(t, err)

	gotArtist, err := svc.GetArtist(artist.ID.String())
	require.NoError(t, err)
	assert.Equal(t, "Bowie", gotArtist.Name)

	albums, err := svc.ListArtistAlbums(artist.ID.String(), 1, 50)
	require.NoError(t, err)
	assert.Len(t, albums, 1)

	_, err = svc.ListAlbums(AlbumFilter{})
	require.NoError(t, err)
	_, err = svc.CountAlbums("")
	require.NoError(t, err)

	gotAlbum, err := svc.GetAlbum(album.ID.String())
	require.NoError(t, err)
	assert.Equal(t, "Low", gotAlbum.Title)

	tracks, err := svc.ListAlbumTracks(album.ID.String(), 1, 50)
	require.NoError(t, err)
	assert.Len(t, tracks, 1)

	gotTrack, err := svc.GetTrack(track.ID.String())
	require.NoError(t, err)
	assert.Equal(t, "Speed of Life", gotTrack.Title)
}

func TestMusicService_CreateArtistAndDeletes(t *testing.T) {
	artists := &memArtistRepo{}
	albums := &memAlbumRepo{}
	tracks := &memTrackRepo{}
	svc := NewMusicService(artists, albums, tracks)

	a, err := svc.CreateArtist(ArtistInput{Name: "Eno", Bio: "amb"})
	require.NoError(t, err)
	assert.Equal(t, "Eno", a.Name)
	require.NoError(t, svc.DeleteArtist(a.ID.String()))

	al := &models.Album{Base: models.Base{ID: uuid.New()}, Title: "Music for Airports"}
	albums.albums = []*models.Album{al}
	require.NoError(t, svc.DeleteAlbum(al.ID.String()))

	tr := &models.Track{Base: models.Base{ID: uuid.New()}, Title: "1/1"}
	tracks.tracks = []*models.Track{tr}
	require.NoError(t, svc.DeleteTrack(tr.ID.String()))
}

// --- Audiobooks ---

func TestAudiobookService_ReadPassthroughs(t *testing.T) {
	book := &models.Audiobook{Base: models.Base{ID: uuid.New()}, Title: "Dune", Genre: "sci-fi"}
	chapter := &models.AudiobookChapter{Base: models.Base{ID: uuid.New()}, AudiobookID: book.ID, Number: 1, Title: "One"}
	svc := NewAudiobookService(
		&memAudiobookRepo{books: []*models.Audiobook{book}},
		&memChapterRepo{chapters: []*models.AudiobookChapter{chapter}},
		&memCleanupRepo{},
	)

	_, err := svc.List(AudiobookFilter{})
	require.NoError(t, err)
	_, err = svc.Count("")
	require.NoError(t, err)

	got, err := svc.GetByID(book.ID.String())
	require.NoError(t, err)
	assert.Equal(t, "Dune", got.Title)

	gotCh, err := svc.GetChapter(chapter.ID.String())
	require.NoError(t, err)
	assert.Equal(t, "One", gotCh.Title)

	created, err := svc.Create(AudiobookInput{Title: "New Book", Genre: "myth"})
	require.NoError(t, err)
	assert.Equal(t, "New Book", created.Title)
}

func TestAudiobookService_Similar_RanksByGenre(t *testing.T) {
	src := &models.Audiobook{Base: models.Base{ID: uuid.New()}, Title: "Src", Genre: "history"}
	match := &models.Audiobook{Base: models.Base{ID: uuid.New()}, Title: "Match", Genre: "history"}
	miss := &models.Audiobook{Base: models.Base{ID: uuid.New()}, Title: "Miss", Genre: "cooking"}
	svc := NewAudiobookService(
		&memAudiobookRepo{books: []*models.Audiobook{src, match, miss}},
		&memChapterRepo{}, &memCleanupRepo{},
	)

	got, err := svc.Similar(src.ID.String(), 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Match", got[0].Title)
	assert.Equal(t, "audiobook", got[0].Type)
}
