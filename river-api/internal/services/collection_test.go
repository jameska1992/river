package services

import (
	"testing"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectionService_AddItem_Success(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Metropolis", Year: 1927, PosterPath: "/m.jpg"}
	movies := &memMovieRepo{movies: []*models.Movie{movie}}
	cols := &memCollectionRepo{}
	svc := NewCollectionService(cols, movies, &memShowRepo{}, &memAudiobookRepo{})

	col, err := svc.Create("u1", "Classics", "")
	require.NoError(t, err)

	detail, err := svc.AddItem(col.ID.String(), "movie", movie.ID.String())
	require.NoError(t, err)
	assert.Equal(t, "Metropolis", detail.Title)
	assert.Equal(t, 1927, detail.Year)
	assert.Equal(t, "/m.jpg", detail.PosterPath)
	assert.Equal(t, "movie", detail.MediaType)

	// The item was actually persisted to the collection.
	items, err := cols.FindItems(col.ID.String())
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestCollectionService_AddItem_ShowAndAudiobook(t *testing.T) {
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "The Lone Ranger", Year: 1949, PosterPath: "/s.jpg"}
	book := &models.Audiobook{Base: models.Base{ID: uuid.New()}, Title: "Dracula", Year: 1897, CoverPath: "/b.jpg"}
	svc := NewCollectionService(
		&memCollectionRepo{},
		&memMovieRepo{},
		&memShowRepo{shows: []*models.TVShow{show}},
		&memAudiobookRepo{books: []*models.Audiobook{book}},
	)
	col, err := svc.Create("u1", "Mixed", "")
	require.NoError(t, err)

	t.Run("tvshow", func(t *testing.T) {
		detail, err := svc.AddItem(col.ID.String(), "tvshow", show.ID.String())
		require.NoError(t, err)
		assert.Equal(t, "The Lone Ranger", detail.Title)
		assert.Equal(t, "/s.jpg", detail.PosterPath)
		assert.Equal(t, 1949, detail.Year)
	})

	t.Run("audiobook uses cover as poster", func(t *testing.T) {
		detail, err := svc.AddItem(col.ID.String(), "audiobook", book.ID.String())
		require.NoError(t, err)
		assert.Equal(t, "Dracula", detail.Title)
		assert.Equal(t, "/b.jpg", detail.PosterPath, "audiobook CoverPath should surface as PosterPath")
	})
}

func TestCollectionService_AddItem_Duplicate(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Metropolis"}
	svc := NewCollectionService(&memCollectionRepo{}, &memMovieRepo{movies: []*models.Movie{movie}}, &memShowRepo{}, &memAudiobookRepo{})
	col, err := svc.Create("u1", "Classics", "")
	require.NoError(t, err)

	_, err = svc.AddItem(col.ID.String(), "movie", movie.ID.String())
	require.NoError(t, err)

	_, err = svc.AddItem(col.ID.String(), "movie", movie.ID.String())
	assert.ErrorIs(t, err, ErrConflict, "adding the same media twice should conflict")
}

func TestCollectionService_AddItem_MissingMedia(t *testing.T) {
	svc := NewCollectionService(&memCollectionRepo{}, &memMovieRepo{}, &memShowRepo{}, &memAudiobookRepo{})
	col, err := svc.Create("u1", "Classics", "")
	require.NoError(t, err)

	_, err = svc.AddItem(col.ID.String(), "movie", uuid.New().String())
	assert.ErrorIs(t, err, ErrNotFound, "referencing a non-existent movie should be not-found")
}

func TestCollectionService_AddItem_UnknownMediaType(t *testing.T) {
	svc := NewCollectionService(&memCollectionRepo{}, &memMovieRepo{}, &memShowRepo{}, &memAudiobookRepo{})
	col, err := svc.Create("u1", "Classics", "")
	require.NoError(t, err)

	_, err = svc.AddItem(col.ID.String(), "podcast", uuid.New().String())
	assert.ErrorIs(t, err, ErrNotFound, "an unsupported media type should be rejected")
}

func TestCollectionService_AddItem_CollectionNotFound(t *testing.T) {
	svc := NewCollectionService(&memCollectionRepo{}, &memMovieRepo{}, &memShowRepo{}, &memAudiobookRepo{})
	_, err := svc.AddItem(uuid.New().String(), "movie", uuid.New().String())
	assert.ErrorIs(t, err, ErrNotFound)
}
