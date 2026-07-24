package services

import (
	"testing"
	"time"

	"river-api/internal/models"
	"river-api/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memServiceLogRepo struct {
	created    []*models.ServiceLog
	lastFilter repository.ListLogsFilter
	logs       []models.ServiceLog
}

func (m *memServiceLogRepo) Create(e *models.ServiceLog) error {
	m.created = append(m.created, e)
	return nil
}
func (m *memServiceLogRepo) List(f repository.ListLogsFilter) ([]models.ServiceLog, int64, error) {
	m.lastFilter = f
	return m.logs, int64(len(m.logs)), nil
}

func TestServiceLogService_Create(t *testing.T) {
	repo := &memServiceLogRepo{}
	svc := NewServiceLogService(repo)

	require.NoError(t, svc.Create(CreateLogInput{Level: "info", Service: "river-scan", Message: "discovered movie"}))
	require.Len(t, repo.created, 1)
	assert.Equal(t, "info", repo.created[0].Level)
	assert.Equal(t, "river-scan", repo.created[0].Service)
	assert.Equal(t, "discovered movie", repo.created[0].Message)
}

func TestServiceLogService_List_ParsesDates(t *testing.T) {
	repo := &memServiceLogRepo{}
	svc := NewServiceLogService(repo)

	_, _, err := svc.List(ListLogsInput{
		Level: "error", Service: "river-api",
		From: "2020-01-02T03:04:05Z", To: "2021-06-07T08:09:10Z",
		Page: 2, Limit: 20,
	})
	require.NoError(t, err)

	f := repo.lastFilter
	assert.Equal(t, "error", f.Level)
	assert.Equal(t, "river-api", f.Service)
	assert.Equal(t, 20, f.Offset, "page 2 @ limit 20 => offset 20")
	assert.Equal(t, 20, f.Limit)
	assert.Equal(t, time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC), f.From.UTC())
	assert.Equal(t, time.Date(2021, 6, 7, 8, 9, 10, 0, time.UTC), f.To.UTC())
}

func TestServiceLogService_List_IgnoresInvalidDates(t *testing.T) {
	repo := &memServiceLogRepo{}
	svc := NewServiceLogService(repo)

	_, _, err := svc.List(ListLogsInput{From: "not-a-date", To: ""})
	require.NoError(t, err)
	assert.True(t, repo.lastFilter.From.IsZero(), "an unparseable From should be left as the zero time")
	assert.True(t, repo.lastFilter.To.IsZero())
}

func TestServiceLogService_List_DefaultPagination(t *testing.T) {
	repo := &memServiceLogRepo{}
	svc := NewServiceLogService(repo)

	_, _, err := svc.List(ListLogsInput{})
	require.NoError(t, err)
	assert.Equal(t, 0, repo.lastFilter.Offset)
	assert.Equal(t, 50, repo.lastFilter.Limit, "default page size is 50")
}
