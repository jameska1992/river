package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"river-api/internal/models"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// audiobookDeleteRouter wires the audiobook handler with a real mediaBase so
// the delete_files path can exercise on-disk removal.
func audiobookDeleteRouter(books *fakeAudiobookRepo, chapters *fakeChapterRepo, mediaBase string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := services.NewAudiobookService(books, chapters, fakeCleanupRepo{})
	h := NewAudiobookHandler(svc, "", mediaBase)
	r := gin.New()
	r.DELETE("/audiobooks/:id", h.Delete)
	r.DELETE("/audiobooks/:id/chapters/:chapterId", h.DeleteChapter)
	return r
}

// writeChapterFile creates a file under dir and returns its path.
func writeChapterFile(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte("x"), 0o644))
	return p
}

func TestAudiobookHandler_Delete_RowOnlyKeepsFiles(t *testing.T) {
	base := t.TempDir()
	f1 := writeChapterFile(t, base, "Dracula/ch1.m4a")

	bookID := uuid.New()
	books := &fakeAudiobookRepo{books: []*models.Audiobook{{Base: models.Base{ID: bookID}, Title: "Dracula"}}}
	chapters := &fakeChapterRepo{chapters: []*models.AudiobookChapter{
		{Base: models.Base{ID: uuid.New()}, AudiobookID: bookID, Number: 1, FilePath: f1},
	}}
	r := audiobookDeleteRouter(books, chapters, base)

	w := doJSON(r, http.MethodDelete, "/audiobooks/"+bookID.String(), "")
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, books.books, "book row removed")
	assert.FileExists(t, f1, "file must be left on disk when delete_files is not set")
}

func TestAudiobookHandler_Delete_WithFilesRemovesFromDisk(t *testing.T) {
	base := t.TempDir()
	f1 := writeChapterFile(t, base, "Dracula/ch1.m4a")
	f2 := writeChapterFile(t, base, "Dracula/ch2.m4a")

	bookID := uuid.New()
	books := &fakeAudiobookRepo{books: []*models.Audiobook{{Base: models.Base{ID: bookID}, Title: "Dracula"}}}
	chapters := &fakeChapterRepo{chapters: []*models.AudiobookChapter{
		{Base: models.Base{ID: uuid.New()}, AudiobookID: bookID, Number: 1, FilePath: f1},
		{Base: models.Base{ID: uuid.New()}, AudiobookID: bookID, Number: 2, FilePath: f2},
	}}
	r := audiobookDeleteRouter(books, chapters, base)

	w := doJSON(r, http.MethodDelete, "/audiobooks/"+bookID.String()+"?delete_files=true", "")
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, books.books, "book row removed")
	assert.NoFileExists(t, f1, "chapter file removed from disk")
	assert.NoFileExists(t, f2, "chapter file removed from disk")
}

func TestAudiobookHandler_DeleteChapter(t *testing.T) {
	base := t.TempDir()
	f1 := writeChapterFile(t, base, "Dracula/ch1.m4a")

	bookID, chID := uuid.New(), uuid.New()
	newChapters := func() *fakeChapterRepo {
		return &fakeChapterRepo{chapters: []*models.AudiobookChapter{
			{Base: models.Base{ID: chID}, AudiobookID: bookID, Number: 1, FilePath: f1},
		}}
	}

	t.Run("unknown chapter is 404", func(t *testing.T) {
		chapters := newChapters()
		r := audiobookDeleteRouter(&fakeAudiobookRepo{}, chapters, base)
		w := doJSON(r, http.MethodDelete, "/audiobooks/"+bookID.String()+"/chapters/"+uuid.New().String(), "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("row-only keeps file", func(t *testing.T) {
		require.NoError(t, os.WriteFile(f1, []byte("x"), 0o644))
		chapters := newChapters()
		r := audiobookDeleteRouter(&fakeAudiobookRepo{}, chapters, base)
		w := doJSON(r, http.MethodDelete, "/audiobooks/"+bookID.String()+"/chapters/"+chID.String(), "")
		require.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, chapters.chapters, "chapter row removed")
		assert.FileExists(t, f1, "file kept when delete_files unset")
	})

	t.Run("delete_files removes the file", func(t *testing.T) {
		require.NoError(t, os.WriteFile(f1, []byte("x"), 0o644))
		chapters := newChapters()
		r := audiobookDeleteRouter(&fakeAudiobookRepo{}, chapters, base)
		w := doJSON(r, http.MethodDelete, "/audiobooks/"+bookID.String()+"/chapters/"+chID.String()+"?delete_files=true", "")
		require.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, chapters.chapters, "chapter row removed")
		assert.NoFileExists(t, f1, "file removed from disk")
	})
}
