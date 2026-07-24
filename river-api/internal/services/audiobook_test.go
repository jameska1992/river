package services

import (
	"errors"
	"testing"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAudiobookService_Delete_PurgesThenDeletes(t *testing.T) {
	book := &models.Audiobook{Base: models.Base{ID: uuid.New()}, Title: "Dracula"}
	repo := &memAudiobookRepo{books: []*models.Audiobook{book}}
	cleanup := &memCleanupRepo{}
	svc := NewAudiobookService(repo, &memChapterRepo{}, cleanup)

	require.NoError(t, svc.Delete(book.ID.String()))
	assert.Contains(t, cleanup.purged, "audiobook:"+book.ID.String())
	_, err := repo.FindByID(book.ID.String())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestAudiobookService_Delete_PurgeErrorAborts(t *testing.T) {
	book := &models.Audiobook{Base: models.Base{ID: uuid.New()}, Title: "Stays"}
	repo := &memAudiobookRepo{books: []*models.Audiobook{book}}
	svc := NewAudiobookService(repo, &memChapterRepo{}, &memCleanupRepo{audiobookErr: errors.New("purge failed")})

	assert.Error(t, svc.Delete(book.ID.String()))
	_, err := repo.FindByID(book.ID.String())
	assert.NoError(t, err, "the audiobook must not be deleted when purge fails")
}

func TestAudiobookService_Similar_NoGenreIsEmpty(t *testing.T) {
	book := &models.Audiobook{Base: models.Base{ID: uuid.New()}, Genre: ""}
	repo := &memAudiobookRepo{books: []*models.Audiobook{book}}
	svc := NewAudiobookService(repo, &memChapterRepo{}, &memCleanupRepo{})

	got, err := svc.Similar(book.ID.String(), 10)
	require.NoError(t, err)
	assert.Empty(t, got, "an audiobook with no genre has no similar titles")
}

func TestAudiobookService_CreateChapter_Success(t *testing.T) {
	book := &models.Audiobook{Base: models.Base{ID: uuid.New()}, Title: "Dracula"}
	chapters := &memChapterRepo{}
	svc := NewAudiobookService(&memAudiobookRepo{books: []*models.Audiobook{book}}, chapters, &memCleanupRepo{})

	ch, err := svc.CreateChapter(book.ID.String(), ChapterInput{Number: 1, Title: "Chapter 1"})
	require.NoError(t, err)
	assert.Equal(t, book.ID, ch.AudiobookID, "chapter should be linked to its audiobook")
	assert.Equal(t, 1, ch.Number)

	list, err := svc.ListChapters(book.ID.String())
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestAudiobookService_CreateChapter_BookNotFound(t *testing.T) {
	svc := NewAudiobookService(&memAudiobookRepo{}, &memChapterRepo{}, &memCleanupRepo{})
	_, err := svc.CreateChapter(uuid.New().String(), ChapterInput{Number: 1})
	assert.ErrorIs(t, err, ErrNotFound, "creating a chapter under a missing audiobook should fail")
}

func TestAudiobookService_Update(t *testing.T) {
	book := &models.Audiobook{Base: models.Base{ID: uuid.New()}, Title: "Old", Author: "A"}
	repo := &memAudiobookRepo{books: []*models.Audiobook{book}}
	svc := NewAudiobookService(repo, &memChapterRepo{}, &memCleanupRepo{})

	t.Run("updates fields", func(t *testing.T) {
		updated, err := svc.Update(book.ID.String(), AudiobookInput{Title: "New", Author: "B", Year: 1897})
		require.NoError(t, err)
		assert.Equal(t, "New", updated.Title)
		assert.Equal(t, "B", updated.Author)
		assert.Equal(t, 1897, updated.Year)
	})

	t.Run("missing is not found", func(t *testing.T) {
		_, err := svc.Update(uuid.New().String(), AudiobookInput{Title: "X"})
		assert.ErrorIs(t, err, ErrNotFound)
	})
}
