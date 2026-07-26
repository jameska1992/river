package services

import (
	"testing"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func collItem(colID, mediaType, mediaID string) *models.CollectionItem {
	return &models.CollectionItem{
		Base:         models.Base{ID: uuid.New()},
		CollectionID: colID,
		MediaType:    mediaType,
		MediaID:      mediaID,
	}
}

func TestCollectionService_List_ResolvesCoversAndCounts(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "M", PosterPath: "/m.jpg"}
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "S", PosterPath: "/s.jpg"}
	book := &models.Audiobook{Base: models.Base{ID: uuid.New()}, Title: "B", CoverPath: "/b.jpg"}

	col := &models.Collection{Base: models.Base{ID: uuid.New()}, Name: "Mixed"}
	items := []*models.CollectionItem{
		collItem(col.ID.String(), "movie", movie.ID.String()),
		collItem(col.ID.String(), "tvshow", show.ID.String()),
		collItem(col.ID.String(), "audiobook", book.ID.String()),
		// A stale reference: counted in ItemCount, but contributes no cover.
		collItem(col.ID.String(), "movie", uuid.NewString()),
	}
	cols := &memCollectionRepo{cols: []*models.Collection{col}, items: items}
	svc := NewCollectionService(cols,
		&memMovieRepo{movies: []*models.Movie{movie}},
		&memShowRepo{shows: []*models.TVShow{show}},
		&memAudiobookRepo{books: []*models.Audiobook{book}},
	)

	got, err := svc.List()
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 4, got[0].ItemCount, "stale item still counts")
	assert.ElementsMatch(t, []string{"/m.jpg", "/s.jpg", "/b.jpg"}, got[0].Covers,
		"covers resolve from each media type; the stale reference is skipped")
}

func TestCollectionService_List_CapsCoversAtFour(t *testing.T) {
	col := &models.Collection{Base: models.Base{ID: uuid.New()}, Name: "Big"}
	movies := make([]*models.Movie, 0, 6)
	items := make([]*models.CollectionItem, 0, 6)
	for i := 0; i < 6; i++ {
		m := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "M", PosterPath: "/p.jpg"}
		movies = append(movies, m)
		items = append(items, collItem(col.ID.String(), "movie", m.ID.String()))
	}
	svc := NewCollectionService(
		&memCollectionRepo{cols: []*models.Collection{col}, items: items},
		&memMovieRepo{movies: movies}, &memShowRepo{}, &memAudiobookRepo{},
	)

	got, err := svc.List()
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 6, got[0].ItemCount)
	assert.Len(t, got[0].Covers, 4, "covers are capped at four")
}

func TestCollectionService_GetWithItems(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Metropolis", Year: 1927, PosterPath: "/m.jpg"}
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "Firefly", Year: 2002, PosterPath: "/s.jpg"}
	book := &models.Audiobook{Base: models.Base{ID: uuid.New()}, Title: "Dracula", Year: 1897, CoverPath: "/b.jpg"}
	col := &models.Collection{Base: models.Base{ID: uuid.New()}, Name: "Faves"}
	items := []*models.CollectionItem{
		collItem(col.ID.String(), "movie", movie.ID.String()),
		collItem(col.ID.String(), "tvshow", show.ID.String()),
		collItem(col.ID.String(), "audiobook", book.ID.String()),
	}
	svc := NewCollectionService(
		&memCollectionRepo{cols: []*models.Collection{col}, items: items},
		&memMovieRepo{movies: []*models.Movie{movie}},
		&memShowRepo{shows: []*models.TVShow{show}},
		&memAudiobookRepo{books: []*models.Audiobook{book}},
	)

	detail, err := svc.GetWithItems(col.ID.String())
	require.NoError(t, err)
	assert.Equal(t, "Faves", detail.Name)
	require.Len(t, detail.Items, 3)
	assert.Equal(t, "Metropolis", detail.Items[0].Title)
	assert.Equal(t, 1927, detail.Items[0].Year)
	assert.Equal(t, "Firefly", detail.Items[1].Title)
	assert.Equal(t, "/s.jpg", detail.Items[1].PosterPath)
	assert.Equal(t, "Dracula", detail.Items[2].Title)
	assert.Equal(t, "/b.jpg", detail.Items[2].PosterPath, "audiobook cover surfaces as poster")

	_, err = svc.GetWithItems(uuid.NewString())
	assert.Error(t, err, "unknown collection is not-found")
}

func TestCollectionService_Update(t *testing.T) {
	col := &models.Collection{Base: models.Base{ID: uuid.New()}, Name: "Old", Description: "old"}
	svc := NewCollectionService(
		&memCollectionRepo{cols: []*models.Collection{col}},
		&memMovieRepo{}, &memShowRepo{}, &memAudiobookRepo{},
	)

	got, err := svc.Update(col.ID.String(), "New", "new desc")
	require.NoError(t, err)
	assert.Equal(t, "New", got.Name)
	assert.Equal(t, "new desc", got.Description)

	_, err = svc.Update(uuid.NewString(), "x", "y")
	assert.Error(t, err)
}

func TestCollectionService_Delete(t *testing.T) {
	col := &models.Collection{Base: models.Base{ID: uuid.New()}, Name: "Doomed"}
	cols := &memCollectionRepo{cols: []*models.Collection{col}}
	svc := NewCollectionService(cols, &memMovieRepo{}, &memShowRepo{}, &memAudiobookRepo{})

	require.NoError(t, svc.Delete(col.ID.String()))
	assert.Empty(t, cols.cols)

	assert.Error(t, svc.Delete(uuid.NewString()), "deleting a missing collection errors")
}

func TestCollectionService_RemoveItem(t *testing.T) {
	col := &models.Collection{Base: models.Base{ID: uuid.New()}, Name: "C"}
	item := collItem(col.ID.String(), "movie", uuid.NewString())
	other := &models.Collection{Base: models.Base{ID: uuid.New()}, Name: "Other"}
	// An item that belongs to a different collection than the one asked about.
	foreign := collItem(other.ID.String(), "movie", uuid.NewString())

	cols := &memCollectionRepo{
		cols:  []*models.Collection{col, other},
		items: []*models.CollectionItem{item, foreign},
	}
	svc := NewCollectionService(cols, &memMovieRepo{}, &memShowRepo{}, &memAudiobookRepo{})

	require.NoError(t, svc.RemoveItem(col.ID.String(), item.ID.String()))
	remaining, _ := cols.FindItems(col.ID.String())
	assert.Empty(t, remaining)

	// Unknown collection.
	assert.Error(t, svc.RemoveItem(uuid.NewString(), item.ID.String()))

	// Unknown item.
	assert.Error(t, svc.RemoveItem(col.ID.String(), uuid.NewString()))

	// Item exists but belongs to a different collection → not-found.
	err := svc.RemoveItem(col.ID.String(), foreign.ID.String())
	assert.ErrorIs(t, err, ErrNotFound)
}
