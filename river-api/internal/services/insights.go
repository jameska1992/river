package services

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"river-api/internal/repository"
)

// topTitlesLimit caps how many titles the "most watched" list returns. Kept
// small — this is a dashboard highlight, not a browsable list.
const topTitlesLimit = 10

// InsightsService turns the raw watch_progress aggregates from the repository
// into a display-ready dashboard payload: it resolves media ids to titles and
// user ids to usernames, and derives the completion rate. All heavy lifting
// (SUM/GROUP BY) happens in the repository; this layer only enriches.
type InsightsService struct {
	repo       repository.InsightsRepository
	movies     repository.MovieRepository
	episodes   repository.EpisodeRepository
	shows      repository.TVShowRepository
	audiobooks repository.AudiobookRepository
	chapters   repository.ChapterRepository
	users      repository.UserRepository
}

func NewInsightsService(
	repo repository.InsightsRepository,
	movies repository.MovieRepository,
	episodes repository.EpisodeRepository,
	shows repository.TVShowRepository,
	audiobooks repository.AudiobookRepository,
	chapters repository.ChapterRepository,
	users repository.UserRepository,
) *InsightsService {
	return &InsightsService{
		repo: repo, movies: movies, episodes: episodes, shows: shows,
		audiobooks: audiobooks, chapters: chapters, users: users,
	}
}

// WatchInsights is the full watch-analytics payload for a time window.
type WatchInsights struct {
	// Window is the normalized window that was applied ("7d", "30d", "all"…),
	// echoed back so the client can confirm what it got.
	Window            string             `json:"window"`
	TotalWatchSeconds float64            `json:"total_watch_seconds"`
	Started           int64              `json:"started"`
	Completed         int64              `json:"completed"`
	CompletionRate    float64            `json:"completion_rate"`
	TopTitles         []TopTitleItem     `json:"top_titles"`
	Activity          []WatchActivityDay `json:"activity"`
	PerUser           []PerUserWatchItem `json:"per_user"`
}

// TopTitleItem is one most-watched title with its display metadata resolved.
type TopTitleItem struct {
	MediaType    string  `json:"media_type"`
	MediaID      string  `json:"media_id"`
	Title        string  `json:"title"`
	ShowTitle    string  `json:"show_title,omitempty"`
	Plays        int64   `json:"plays"`
	Completions  int64   `json:"completions"`
	WatchSeconds float64 `json:"watch_seconds"`
}

// WatchActivityDay is watch activity for a single day, for the sparkline/bars.
type WatchActivityDay struct {
	Date         string  `json:"date"`
	WatchSeconds float64 `json:"watch_seconds"`
	Plays        int64   `json:"plays"`
}

// PerUserWatchItem is per-user watch time with the username resolved.
type PerUserWatchItem struct {
	UserID       string  `json:"user_id"`
	Username     string  `json:"username"`
	WatchSeconds float64 `json:"watch_seconds"`
	ItemCount    int64   `json:"item_count"`
}

// LibraryInsights is the library & storage health payload.
type LibraryInsights struct {
	Libraries         []repository.LibraryHealth `json:"libraries"`
	TotalSizeBytes    int64                      `json:"total_size_bytes"`
	UntranscodedTotal int64                      `json:"untranscoded_total"`
	FailedJobs        int64                      `json:"failed_jobs"`
}

// LibraryInsights returns per-library counts, storage usage, and untranscoded/
// failed tallies for the admin storage-health view.
func (s *InsightsService) LibraryInsights() (*LibraryInsights, error) {
	libs, err := s.repo.LibraryBreakdown()
	if err != nil {
		return nil, err
	}
	failed, err := s.repo.FailedJobCount()
	if err != nil {
		return nil, err
	}
	out := &LibraryInsights{Libraries: libs, FailedJobs: failed}
	for _, l := range libs {
		out.TotalSizeBytes += l.SizeBytes
		out.UntranscodedTotal += l.Untranscoded
	}
	return out, nil
}

// WatchInsights aggregates watch analytics for the given window. An empty or
// unrecognized window falls back to the last 30 days.
func (s *InsightsService) WatchInsights(window string) (*WatchInsights, error) {
	since, normalized := parseWatchWindow(window)

	totals, err := s.repo.WatchTotals(since)
	if err != nil {
		return nil, err
	}
	top, err := s.repo.TopTitles(since, topTitlesLimit)
	if err != nil {
		return nil, err
	}
	perUser, err := s.repo.PerUserWatch(since)
	if err != nil {
		return nil, err
	}
	activity, err := s.repo.ActivityByDay(since)
	if err != nil {
		return nil, err
	}

	out := &WatchInsights{
		Window:            normalized,
		TotalWatchSeconds: totals.TotalWatchSeconds,
		Started:           totals.Started,
		Completed:         totals.Completed,
		TopTitles:         s.resolveTopTitles(top),
		PerUser:           s.resolvePerUser(perUser),
		Activity:          make([]WatchActivityDay, 0, len(activity)),
	}
	if totals.Started > 0 {
		out.CompletionRate = float64(totals.Completed) / float64(totals.Started)
	}
	for _, b := range activity {
		out.Activity = append(out.Activity, WatchActivityDay{
			Date: b.Date, WatchSeconds: b.WatchSeconds, Plays: b.Plays,
		})
	}
	return out, nil
}

// parseWatchWindow maps a window string to a lower time bound and a normalized
// label. Accepts "all" (no lower bound) or "<n>d" (last n days). Anything else
// — including empty — defaults to the last 30 days.
func parseWatchWindow(window string) (time.Time, string) {
	w := strings.ToLower(strings.TrimSpace(window))
	if w == "all" {
		return time.Time{}, "all"
	}
	if strings.HasSuffix(w, "d") {
		if n, err := strconv.Atoi(strings.TrimSuffix(w, "d")); err == nil && n > 0 {
			return time.Now().AddDate(0, 0, -n), fmt.Sprintf("%dd", n)
		}
	}
	return time.Now().AddDate(0, 0, -30), "30d"
}

// resolveTopTitles enriches each aggregate row with its title/parent title,
// dropping rows whose media has since been deleted (stale progress).
func (s *InsightsService) resolveTopTitles(rows []repository.TopTitle) []TopTitleItem {
	out := make([]TopTitleItem, 0, len(rows))
	for _, r := range rows {
		item := TopTitleItem{
			MediaType:    r.MediaType,
			MediaID:      r.MediaID,
			Plays:        r.Plays,
			Completions:  r.Completions,
			WatchSeconds: r.WatchSeconds,
		}
		switch r.MediaType {
		case "movie":
			m, err := s.movies.FindByID(r.MediaID)
			if err != nil {
				continue
			}
			item.Title = m.Title
		case "episode":
			ep, err := s.episodes.FindByID(r.MediaID)
			if err != nil {
				continue
			}
			show, err := s.shows.FindByID(ep.TVShowID.String())
			if err != nil {
				continue
			}
			if ep.Title != "" {
				item.Title = ep.Title
			} else {
				item.Title = fmt.Sprintf("Episode %d", ep.Number)
			}
			item.ShowTitle = show.Title
		case "chapter":
			ch, err := s.chapters.FindByID(r.MediaID)
			if err != nil {
				continue
			}
			book, err := s.audiobooks.FindByID(ch.AudiobookID.String())
			if err != nil {
				continue
			}
			if ch.Title != "" {
				item.Title = ch.Title
			} else {
				item.Title = fmt.Sprintf("Chapter %d", ch.Number)
			}
			item.ShowTitle = book.Title
		default:
			item.Title = r.MediaType
		}
		out = append(out, item)
	}
	return out
}

// resolvePerUser attaches usernames to the per-user aggregates. A row whose
// user no longer exists is kept (the watch time is still real) but left with an
// empty username rather than dropped.
func (s *InsightsService) resolvePerUser(rows []repository.UserWatch) []PerUserWatchItem {
	out := make([]PerUserWatchItem, 0, len(rows))
	for _, r := range rows {
		item := PerUserWatchItem{
			UserID:       r.UserID,
			WatchSeconds: r.WatchSeconds,
			ItemCount:    r.ItemCount,
		}
		if u, err := s.users.FindByID(r.UserID); err == nil {
			item.Username = u.Username
		}
		out = append(out, item)
	}
	return out
}
