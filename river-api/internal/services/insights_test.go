package services

import (
	"testing"
	"time"

	"river-api/internal/models"
	"river-api/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeInsightsRepo returns canned aggregates and records the `since` bound the
// service computed, so window parsing can be asserted without a real DB.
type fakeInsightsRepo struct {
	totals    repository.WatchTotals
	top       []repository.TopTitle
	perUser   []repository.UserWatch
	activity  []repository.DayBucket
	libraries []repository.LibraryHealth
	failed    int64
	sinceSeen time.Time
}

func (f *fakeInsightsRepo) WatchTotals(since time.Time) (repository.WatchTotals, error) {
	f.sinceSeen = since
	return f.totals, nil
}
func (f *fakeInsightsRepo) TopTitles(since time.Time, limit int) ([]repository.TopTitle, error) {
	return f.top, nil
}
func (f *fakeInsightsRepo) PerUserWatch(since time.Time) ([]repository.UserWatch, error) {
	return f.perUser, nil
}
func (f *fakeInsightsRepo) ActivityByDay(since time.Time) ([]repository.DayBucket, error) {
	return f.activity, nil
}
func (f *fakeInsightsRepo) LibraryBreakdown() ([]repository.LibraryHealth, error) {
	return f.libraries, nil
}
func (f *fakeInsightsRepo) FailedJobCount() (int64, error) {
	return f.failed, nil
}

func newInsightsService(repo repository.InsightsRepository, opts ...func(*insightsFixtures)) *InsightsService {
	f := &insightsFixtures{
		movies:     &memMovieRepo{},
		episodes:   &memEpisodeRepo{},
		shows:      &memShowRepo{},
		audiobooks: &memAudiobookRepo{},
		chapters:   &memChapterRepo{},
		users:      &memUserRepo{},
	}
	for _, o := range opts {
		o(f)
	}
	return NewInsightsService(repo, f.movies, f.episodes, f.shows, f.audiobooks, f.chapters, f.users)
}

type insightsFixtures struct {
	movies     *memMovieRepo
	episodes   *memEpisodeRepo
	shows      *memShowRepo
	audiobooks *memAudiobookRepo
	chapters   *memChapterRepo
	users      *memUserRepo
}

func TestInsightsService_WindowParsing(t *testing.T) {
	cases := []struct {
		in          string
		wantLabel   string
		wantAllTime bool
	}{
		{"all", "all", true},
		{"7d", "7d", false},
		{"30d", "30d", false},
		{"90d", "90d", false},
		{"", "30d", false},
		{"nonsense", "30d", false},
		{"0d", "30d", false}, // non-positive → default
	}
	for _, tc := range cases {
		repo := &fakeInsightsRepo{}
		svc := newInsightsService(repo)
		out, err := svc.WatchInsights(tc.in)
		require.NoError(t, err)
		assert.Equal(t, tc.wantLabel, out.Window, "window %q", tc.in)
		if tc.wantAllTime {
			assert.True(t, repo.sinceSeen.IsZero(), "all → zero lower bound")
		} else {
			assert.False(t, repo.sinceSeen.IsZero(), "window %q → non-zero bound", tc.in)
		}
	}
}

func TestInsightsService_CompletionRate(t *testing.T) {
	repo := &fakeInsightsRepo{totals: repository.WatchTotals{TotalWatchSeconds: 500, Started: 4, Completed: 1}}
	out, err := newInsightsService(repo).WatchInsights("30d")
	require.NoError(t, err)
	assert.Equal(t, float64(500), out.TotalWatchSeconds)
	assert.Equal(t, 0.25, out.CompletionRate)

	// No starts → rate is 0, not a divide-by-zero.
	repoEmpty := &fakeInsightsRepo{}
	outEmpty, err := newInsightsService(repoEmpty).WatchInsights("30d")
	require.NoError(t, err)
	assert.Equal(t, float64(0), outEmpty.CompletionRate)
	assert.Empty(t, outEmpty.TopTitles)
	assert.Empty(t, outEmpty.PerUser)
}

func TestInsightsService_ResolvesTopTitles(t *testing.T) {
	movieID := uuid.New()
	showID := uuid.New()
	epID := uuid.New()
	staleID := uuid.New()

	repo := &fakeInsightsRepo{top: []repository.TopTitle{
		{MediaType: "movie", MediaID: movieID.String(), Plays: 3, Completions: 2, WatchSeconds: 300},
		{MediaType: "episode", MediaID: epID.String(), Plays: 1, WatchSeconds: 40},
		{MediaType: "movie", MediaID: staleID.String(), Plays: 9, WatchSeconds: 900}, // deleted media → dropped
	}}
	svc := newInsightsService(repo, func(f *insightsFixtures) {
		f.movies.movies = []*models.Movie{{Base: models.Base{ID: movieID}, Title: "Inception"}}
		f.shows.shows = []*models.TVShow{{Base: models.Base{ID: showID}, Title: "The Wire"}}
		f.episodes.episodes = []*models.Episode{{Base: models.Base{ID: epID}, TVShowID: showID, Number: 1, Title: "The Target"}}
	})

	out, err := svc.WatchInsights("all")
	require.NoError(t, err)
	require.Len(t, out.TopTitles, 2) // stale row dropped

	assert.Equal(t, "Inception", out.TopTitles[0].Title)
	assert.Equal(t, int64(3), out.TopTitles[0].Plays)

	assert.Equal(t, "The Target", out.TopTitles[1].Title)
	assert.Equal(t, "The Wire", out.TopTitles[1].ShowTitle)
}

func TestInsightsService_LibraryInsights(t *testing.T) {
	repo := &fakeInsightsRepo{
		libraries: []repository.LibraryHealth{
			{ID: "1", Name: "Films", Type: "movie", ItemCount: 3, SizeBytes: 300, Untranscoded: 1},
			{ID: "2", Name: "Shows", Type: "tvshow", ItemCount: 1, SizeBytes: 120, Untranscoded: 2},
		},
		failed: 5,
	}
	out, err := newInsightsService(repo).LibraryInsights()
	require.NoError(t, err)

	require.Len(t, out.Libraries, 2)
	assert.Equal(t, int64(420), out.TotalSizeBytes)  // 300 + 120
	assert.Equal(t, int64(3), out.UntranscodedTotal) // 1 + 2
	assert.Equal(t, int64(5), out.FailedJobs)
}

func TestInsightsService_ResolvesPerUser(t *testing.T) {
	u := &models.User{Base: models.Base{ID: uuid.New()}, Username: "alice"}
	missingID := uuid.New()

	repo := &fakeInsightsRepo{perUser: []repository.UserWatch{
		{UserID: u.ID.String(), WatchSeconds: 500, ItemCount: 3},
		{UserID: missingID.String(), WatchSeconds: 100, ItemCount: 1}, // user gone → kept, blank name
	}}
	svc := newInsightsService(repo, func(f *insightsFixtures) {
		f.users.users = []*models.User{u}
	})

	out, err := svc.WatchInsights("all")
	require.NoError(t, err)
	require.Len(t, out.PerUser, 2)

	assert.Equal(t, "alice", out.PerUser[0].Username)
	assert.Equal(t, float64(500), out.PerUser[0].WatchSeconds)

	assert.Equal(t, "", out.PerUser[1].Username)
	assert.Equal(t, missingID.String(), out.PerUser[1].UserID)
}
