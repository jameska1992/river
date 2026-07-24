package services

import (
	"testing"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMusicService(artists *memArtistRepo, albums *memAlbumRepo, tracks *memTrackRepo) *MusicService {
	return NewMusicService(artists, albums, tracks)
}

func TestMusicService_UpdateArtist(t *testing.T) {
	artist := &models.Artist{Base: models.Base{ID: uuid.New()}, Name: "Old", Bio: "b"}
	repo := &memArtistRepo{artists: []*models.Artist{artist}}
	svc := newMusicService(repo, &memAlbumRepo{}, &memTrackRepo{})

	t.Run("updates fields", func(t *testing.T) {
		updated, err := svc.UpdateArtist(artist.ID.String(), ArtistInput{Name: "New", Bio: "new bio"})
		require.NoError(t, err)
		assert.Equal(t, "New", updated.Name)
		assert.Equal(t, "new bio", updated.Bio)
	})

	t.Run("missing is not found", func(t *testing.T) {
		_, err := svc.UpdateArtist(uuid.New().String(), ArtistInput{Name: "X"})
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestMusicService_UpdateAlbum_PreservesArtist(t *testing.T) {
	artistID := uuid.New()
	album := &models.Album{Base: models.Base{ID: uuid.New()}, ArtistID: artistID, Title: "Old", Year: 1990}
	repo := &memAlbumRepo{albums: []*models.Album{album}}
	svc := newMusicService(&memArtistRepo{}, repo, &memTrackRepo{})

	// UpdateAlbum edits metadata but must NOT reassign the album's artist,
	// even if the input carries a different ArtistID.
	updated, err := svc.UpdateAlbum(album.ID.String(), AlbumInput{Title: "New", Year: 1991, ArtistID: uuid.New()})
	require.NoError(t, err)
	assert.Equal(t, "New", updated.Title)
	assert.Equal(t, 1991, updated.Year)
	assert.Equal(t, artistID, updated.ArtistID, "album update must not reassign the artist")
}

func TestMusicService_CreateAlbum_StoresFields(t *testing.T) {
	repo := &memAlbumRepo{}
	svc := newMusicService(&memArtistRepo{}, repo, &memTrackRepo{})
	artistID := uuid.New()

	album, err := svc.CreateAlbum(AlbumInput{ArtistID: artistID, Title: "Kind of Blue", Year: 1959})
	require.NoError(t, err)
	assert.Equal(t, "Kind of Blue", album.Title)
	assert.Equal(t, artistID, album.ArtistID)
	assert.Len(t, repo.albums, 1)
}

func TestMusicService_CreateTrack_StoresFields(t *testing.T) {
	repo := &memTrackRepo{}
	svc := newMusicService(&memArtistRepo{}, &memAlbumRepo{}, repo)
	albumID := uuid.New()

	track, err := svc.CreateTrack(TrackInput{AlbumID: albumID, Title: "So What", Number: 1})
	require.NoError(t, err)
	assert.Equal(t, "So What", track.Title)
	assert.Equal(t, albumID, track.AlbumID)
	assert.Equal(t, 1, track.Number)
}

func TestMusicService_DeleteArtist(t *testing.T) {
	artist := &models.Artist{Base: models.Base{ID: uuid.New()}, Name: "Gone"}
	repo := &memArtistRepo{artists: []*models.Artist{artist}}
	svc := newMusicService(repo, &memAlbumRepo{}, &memTrackRepo{})

	require.NoError(t, svc.DeleteArtist(artist.ID.String()))
	_, err := repo.FindByID(artist.ID.String())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestMusicService_ListArtistAlbums(t *testing.T) {
	artistID := uuid.New()
	albums := &memAlbumRepo{albums: []*models.Album{
		{Base: models.Base{ID: uuid.New()}, ArtistID: artistID, Title: "A"},
		{Base: models.Base{ID: uuid.New()}, ArtistID: artistID, Title: "B"},
		{Base: models.Base{ID: uuid.New()}, ArtistID: uuid.New(), Title: "Other"},
	}}
	svc := newMusicService(&memArtistRepo{}, albums, &memTrackRepo{})

	got, err := svc.ListArtistAlbums(artistID.String(), 1, 50)
	require.NoError(t, err)
	assert.Len(t, got, 2, "only the two albums for this artist should be returned")
}
