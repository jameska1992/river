package handlers

import (
	"net/http"
	"testing"

	"river-api/internal/apperrors"
	"river-api/internal/models"
	"river-api/internal/repository"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeFailedJobRepo is an in-memory FailedJobRepository for handler tests.
type fakeFailedJobRepo struct {
	jobs []*models.FailedJob
}

func (f *fakeFailedJobRepo) Create(job *models.FailedJob) error {
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	f.jobs = append(f.jobs, job)
	return nil
}

func (f *fakeFailedJobRepo) List(_ repository.ListFailedJobsFilter) ([]models.FailedJob, int64, error) {
	out := make([]models.FailedJob, 0, len(f.jobs))
	for _, j := range f.jobs {
		out = append(out, *j)
	}
	return out, int64(len(out)), nil
}

func (f *fakeFailedJobRepo) FindByID(id string) (*models.FailedJob, error) {
	for _, j := range f.jobs {
		if j.ID.String() == id {
			return j, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (f *fakeFailedJobRepo) Delete(id string) error {
	for i, j := range f.jobs {
		if j.ID.String() == id {
			f.jobs = append(f.jobs[:i], f.jobs[i+1:]...)
			return nil
		}
	}
	return apperrors.ErrNotFound
}

func failedJobRouter(t *testing.T, repo *fakeFailedJobRepo, scanURL string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := services.NewFailedJobService(repo)
	h := NewFailedJobHandler(svc, scanURL)
	r := gin.New()
	r.POST("/failed-jobs", h.Report)
	r.GET("/admin/failed-jobs", h.List)
	r.POST("/admin/failed-jobs/:id/retry", h.Retry)
	r.DELETE("/admin/failed-jobs/:id", h.Dismiss)
	return r
}

func TestFailedJob_ReportListDismiss(t *testing.T) {
	repo := &fakeFailedJobRepo{}
	r := failedJobRouter(t, repo, "")

	// Report
	body := `{"service":"river-video-trans","media_type":"movie","source_path":"/m/x.mkv","reason":"boom","attempts":5,"routing_key":"media.discovered.movie","event":"{}"}`
	require.Equal(t, http.StatusNoContent, doJSON(r, http.MethodPost, "/failed-jobs", body).Code)
	require.Len(t, repo.jobs, 1)

	// Missing required service → 400
	assert.Equal(t, http.StatusBadRequest, doJSON(r, http.MethodPost, "/failed-jobs", `{"reason":"x"}`).Code)

	// List
	w := doJSON(r, http.MethodGet, "/admin/failed-jobs", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "river-video-trans")

	// Dismiss
	id := repo.jobs[0].ID.String()
	require.Equal(t, http.StatusNoContent, doJSON(r, http.MethodDelete, "/admin/failed-jobs/"+id, "").Code)
	assert.Empty(t, repo.jobs)

	// Dismiss unknown → 404
	assert.Equal(t, http.StatusNotFound, doJSON(r, http.MethodDelete, "/admin/failed-jobs/"+uuid.NewString(), "").Code)
}

func TestFailedJob_RetryRepublishesAndRemoves(t *testing.T) {
	scan := newStubScan(t)
	repo := &fakeFailedJobRepo{jobs: []*models.FailedJob{{
		Base: models.Base{ID: uuid.New()}, Service: "river-meta-tv",
		Reason: "429", Event: `{"library_type":"tvshow"}`,
	}}}
	id := repo.jobs[0].ID.String()
	r := failedJobRouter(t, repo, scan.srv.URL)

	w := doJSON(r, http.MethodPost, "/admin/failed-jobs/"+id+"/retry", "")
	require.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "/republish", scan.path, "retry delegates to river-scan /republish")
	assert.Empty(t, repo.jobs, "successful retry removes the job")
}

func TestFailedJob_RetryUnknownIs404(t *testing.T) {
	r := failedJobRouter(t, &fakeFailedJobRepo{}, "http://unused")
	assert.Equal(t, http.StatusNotFound, doJSON(r, http.MethodPost, "/admin/failed-jobs/"+uuid.NewString()+"/retry", "").Code)
}
