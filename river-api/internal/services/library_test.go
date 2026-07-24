package services

import (
	"testing"

	"river-api/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLibraryService_CreateDefaultsEmptyPaths(t *testing.T) {
	svc := NewLibraryService(&memLibraryRepo{})

	lib, err := svc.Create(LibraryInput{Name: "Movies", Type: models.LibraryType("movie")})
	require.NoError(t, err)
	assert.Equal(t, "[]", lib.Paths, "empty paths should default to a JSON empty array")
	assert.Equal(t, "Movies", lib.Name)
}

func TestLibraryService_CreatePreservesPaths(t *testing.T) {
	svc := NewLibraryService(&memLibraryRepo{})

	lib, err := svc.Create(LibraryInput{
		Name:  "Movies",
		Type:  models.LibraryType("movie"),
		Paths: `["/srv/media/movies"]`,
	})
	require.NoError(t, err)
	assert.Equal(t, `["/srv/media/movies"]`, lib.Paths)
}

func TestLibraryService_UpdateKeepsPathsWhenBlank(t *testing.T) {
	repo := &memLibraryRepo{}
	svc := NewLibraryService(repo)
	created, err := svc.Create(LibraryInput{
		Name:  "Movies",
		Type:  models.LibraryType("movie"),
		Paths: `["/srv/media/movies"]`,
	})
	require.NoError(t, err)

	// Update with a blank Paths must not wipe the existing paths.
	updated, err := svc.Update(created.ID.String(), LibraryInput{Name: "Films", Type: models.LibraryType("movie")})
	require.NoError(t, err)
	assert.Equal(t, "Films", updated.Name)
	assert.Equal(t, `["/srv/media/movies"]`, updated.Paths, "blank Paths on update should preserve the current value")
}

func TestLibraryService_GetByIDNotFound(t *testing.T) {
	svc := NewLibraryService(&memLibraryRepo{})
	_, err := svc.GetByID("00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, ErrNotFound)
}
