package services

import (
	"testing"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memSearchRepo struct {
	movies []models.Movie
	shows  []models.TVShow
	books  []models.Audiobook
	people []models.Person
}

func (m *memSearchRepo) SearchMovies(q, g string, l int) ([]models.Movie, error) {
	return m.movies, nil
}
func (m *memSearchRepo) SearchTVShows(q, g string, l int) ([]models.TVShow, error) {
	return m.shows, nil
}
func (m *memSearchRepo) SearchAudiobooks(q, g string, l int) ([]models.Audiobook, error) {
	return m.books, nil
}
func (m *memSearchRepo) SearchPeople(q string, l int) ([]models.Person, error) {
	return m.people, nil
}

func movieInLib(libID uuid.UUID, libName, title string) models.Movie {
	return models.Movie{
		Base:      models.Base{ID: uuid.New()},
		LibraryID: libID,
		Library:   models.Library{Name: libName, Type: models.LibraryType("movie")},
		Title:     title,
	}
}

func TestSearchService_GroupsByLibraryPreservingOrder(t *testing.T) {
	movLib := uuid.New()
	showLib := uuid.New()
	repo := &memSearchRepo{
		movies: []models.Movie{movieInLib(movLib, "Films", "A"), movieInLib(movLib, "Films", "B")},
		shows: []models.TVShow{{
			Base: models.Base{ID: uuid.New()}, LibraryID: showLib,
			Library: models.Library{Name: "Shows", Type: models.LibraryType("tvshow")}, Title: "Dragnet",
		}},
	}
	svc := NewSearchService(repo)

	res, err := svc.Search("x", "")
	require.NoError(t, err)
	require.Len(t, res.Libraries, 2, "two distinct libraries")
	// Movies were added first, so the movie library group comes first.
	assert.Equal(t, "Films", res.Libraries[0].LibraryName)
	assert.Len(t, res.Libraries[0].Items, 2)
	assert.Equal(t, "movie", res.Libraries[0].Items[0].MediaType)
	assert.Equal(t, "Shows", res.Libraries[1].LibraryName)
	assert.Equal(t, "tvshow", res.Libraries[1].Items[0].MediaType)
}

func TestSearchService_AudiobookCoverBecomesPoster(t *testing.T) {
	repo := &memSearchRepo{books: []models.Audiobook{{
		Base: models.Base{ID: uuid.New()}, LibraryID: uuid.New(),
		Library: models.Library{Name: "Books", Type: models.LibraryType("audiobook")},
		Title:   "Dracula", CoverPath: "/cover.jpg",
	}}}
	svc := NewSearchService(repo)

	res, err := svc.Search("d", "")
	require.NoError(t, err)
	require.Len(t, res.Libraries, 1)
	item := res.Libraries[0].Items[0]
	assert.Equal(t, "audiobook", item.MediaType)
	assert.Equal(t, "/cover.jpg", item.PosterPath, "audiobook CoverPath should surface as PosterPath")
}

func TestSearchService_SkipsPeopleForGenreOnlySearch(t *testing.T) {
	repo := &memSearchRepo{people: []models.Person{{Base: models.Base{ID: uuid.New()}, Name: "Bela Lugosi"}}}
	svc := NewSearchService(repo)

	t.Run("genre-only search skips people", func(t *testing.T) {
		res, err := svc.Search("", "horror")
		require.NoError(t, err)
		assert.Empty(t, res.People, "people have no genres, so a genre-only search omits them")
	})

	t.Run("text search includes people", func(t *testing.T) {
		res, err := svc.Search("bela", "")
		require.NoError(t, err)
		require.Len(t, res.People, 1)
		assert.Equal(t, "Bela Lugosi", res.People[0].Name)
	})
}
