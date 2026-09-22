package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"river-api/internal/apperrors"
	"river-api/internal/models"
	"river-api/internal/repository"
	"river-api/internal/subdl"

	"github.com/google/uuid"
)

// subdlSearcher is the slice of the SubDL client this service depends on,
// narrowed to an interface so it can be faked in tests.
type subdlSearcher interface {
	Search(ctx context.Context, p subdl.SearchParams) ([]subdl.Subtitle, error)
	Download(ctx context.Context, subURL string) ([]byte, error)
}

// SubtitleSearchService searches an external provider (SubDL) for subtitles of
// a movie/episode and attaches a chosen result as a new Subtitle. It resolves
// the media's TMDB id (and season/episode numbers) to build the provider query,
// downloads + normalises the pick to WebVTT, writes it under
// MEDIA_BASE_PATH/subtitles, and records it via the subtitle repository.
type SubtitleSearchService struct {
	client        subdlSearcher
	settings      *SettingsService
	movies        repository.MovieRepository
	episodes      repository.EpisodeRepository
	seasons       repository.SeasonRepository
	shows         repository.TVShowRepository
	subtitles     repository.SubtitleRepository
	mediaBasePath string
}

func NewSubtitleSearchService(
	client subdlSearcher,
	settings *SettingsService,
	movies repository.MovieRepository,
	episodes repository.EpisodeRepository,
	seasons repository.SeasonRepository,
	shows repository.TVShowRepository,
	subtitles repository.SubtitleRepository,
	mediaBasePath string,
) *SubtitleSearchService {
	return &SubtitleSearchService{
		client: client, settings: settings, movies: movies, episodes: episodes,
		seasons: seasons, shows: shows, subtitles: subtitles, mediaBasePath: mediaBasePath,
	}
}

// SubtitleAttachInput is a chosen search result to download and attach.
type SubtitleAttachInput struct {
	URL      string // the SubDL result URL (path under the download host)
	Language string // BCP-47-ish code stored on the Subtitle, e.g. "en"
	Label    string // human label; defaults to the language when empty
}

func (s *SubtitleSearchService) apiKey() (string, error) {
	k := strings.TrimSpace(s.settings.SubDLKey())
	if k == "" {
		return "", fmt.Errorf("%w: SubDL API key is not configured", apperrors.ErrInvalidInput)
	}
	return k, nil
}

// SearchMovie returns provider results for a movie, keyed on its TMDB id.
func (s *SubtitleSearchService) SearchMovie(ctx context.Context, movieID, languages string) ([]subdl.Subtitle, error) {
	m, err := s.movies.FindByID(movieID)
	if err != nil {
		return nil, err
	}
	if m.TMDBID == 0 {
		return nil, fmt.Errorf("%w: movie has no TMDB id — identify it first", apperrors.ErrInvalidInput)
	}
	key, err := s.apiKey()
	if err != nil {
		return nil, err
	}
	return s.client.Search(ctx, subdl.SearchParams{
		APIKey:    key,
		TmdbID:    m.TMDBID,
		Type:      "movie",
		Languages: normalizeLanguages(languages),
	})
}

// SearchEpisode returns provider results for an episode, keyed on the parent
// show's TMDB id plus the season/episode numbers.
func (s *SubtitleSearchService) SearchEpisode(ctx context.Context, episodeID, languages string) ([]subdl.Subtitle, error) {
	ep, err := s.episodes.FindByID(episodeID)
	if err != nil {
		return nil, err
	}
	show, err := s.shows.FindByID(ep.TVShowID.String())
	if err != nil {
		return nil, err
	}
	if show.TMDBID == 0 {
		return nil, fmt.Errorf("%w: show has no TMDB id — identify it first", apperrors.ErrInvalidInput)
	}
	key, err := s.apiKey()
	if err != nil {
		return nil, err
	}
	seasonNum := 0
	if season, err := s.seasons.FindByID(ep.SeasonID.String()); err == nil {
		seasonNum = season.Number
	}
	return s.client.Search(ctx, subdl.SearchParams{
		APIKey:        key,
		TmdbID:        show.TMDBID,
		Type:          "tv",
		Languages:     normalizeLanguages(languages),
		SeasonNumber:  seasonNum,
		EpisodeNumber: ep.Number,
	})
}

// AttachMovie downloads a chosen result and attaches it to the movie.
func (s *SubtitleSearchService) AttachMovie(ctx context.Context, movieID string, in SubtitleAttachInput) (*models.Subtitle, error) {
	if _, err := s.movies.FindByID(movieID); err != nil {
		return nil, err
	}
	return s.attach(ctx, "movie", movieID, in)
}

// AttachEpisode downloads a chosen result and attaches it to the episode.
func (s *SubtitleSearchService) AttachEpisode(ctx context.Context, episodeID string, in SubtitleAttachInput) (*models.Subtitle, error) {
	if _, err := s.episodes.FindByID(episodeID); err != nil {
		return nil, err
	}
	return s.attach(ctx, "episode", episodeID, in)
}

// attach downloads the archive, normalises it to VTT, writes it to a
// collision-free path under MEDIA_BASE_PATH/subtitles, and records it.
func (s *SubtitleSearchService) attach(ctx context.Context, mediaType, mediaID string, in SubtitleAttachInput) (*models.Subtitle, error) {
	if strings.TrimSpace(in.URL) == "" {
		return nil, fmt.Errorf("%w: subtitle url is required", apperrors.ErrInvalidInput)
	}
	vtt, err := s.client.Download(ctx, in.URL)
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(s.mediaBasePath, "subtitles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create subtitle dir: %w", err)
	}
	// A fresh UUID filename guarantees we never clobber an existing subtitle
	// (the source release name isn't unique and may repeat across attaches).
	path := filepath.Join(dir, uuid.NewString()+".vtt")
	if err := os.WriteFile(path, vtt, 0o644); err != nil {
		return nil, fmt.Errorf("write subtitle: %w", err)
	}

	lang := strings.ToLower(strings.TrimSpace(in.Language))
	if lang == "" {
		lang = "und"
	}
	label := strings.TrimSpace(in.Label)
	if label == "" {
		label = strings.ToUpper(lang)
	}
	sub, err := s.subtitles.Create(repository.SubtitleInput{
		MediaType: mediaType,
		MediaID:   mediaID,
		Language:  lang,
		Label:     label,
		FilePath:  path,
	})
	if err != nil {
		// Best-effort cleanup so a failed DB insert doesn't leave an orphan file.
		_ = os.Remove(path)
		return nil, err
	}
	return sub, nil
}

// normalizeLanguages trims whitespace around a comma-separated language list
// and uppercases the codes (SubDL expects e.g. "EN,FR"). Empty stays empty.
func normalizeLanguages(langs string) string {
	parts := strings.Split(langs, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, strings.ToUpper(t))
		}
	}
	return strings.Join(out, ",")
}
