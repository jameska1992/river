package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"
)

// matchKey normalizes a title for find-or-create lookups so that one writer's
// canonical form lines up with another's. Letters and digits survive
// (lowercased); apostrophes are stripped (so "Don't" ≡ "Dont"); every other
// rune — punctuation, separators, the lot — collapses to whitespace, which
// then folds.
//
// Without this, every external naming convention drifts into its own row:
// TMDB writes "Law & Order: Special Victims Unit" with a colon, the scanner
// parses the folder to "Law & Order Special Victims Unit" without one, and a
// lookup that only handles dot/underscore/hyphen fails to bridge them. Same
// failure mode for "Star Wars: A New Hope" vs "Star Wars - A New Hope", and
// for "Grey's Anatomy" vs "Greys Anatomy". A simple letters+digits canonical
// form bridges them all.
func matchKey(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		case r == '\'' || r == '’': // ASCII + curly apostrophe
			// Skip entirely so the surrounding letters stay one word.
		default:
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

type Library struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Paths         string `json:"paths"` // JSON-encoded []string
	PreTranscoded bool   `json:"pre_transcoded"`
}

func (l Library) ParsedPaths() ([]string, error) {
	var paths []string
	return paths, json.Unmarshal([]byte(l.Paths), &paths)
}

type Movie struct {
	ID            string  `json:"id"`
	LibraryID     string  `json:"library_id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Description   string  `json:"description"`
	Year          int     `json:"year"`
	Genres        string  `json:"genres"`
	Rating        float32 `json:"rating"`
	Runtime       int     `json:"runtime"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	FilePath      string  `json:"file_path"`
	SourcePath    string  `json:"source_path"`
}

type MovieRequest struct {
	LibraryID  string `json:"library_id"`
	Title      string `json:"title"`
	Year       int    `json:"year,omitempty"`
	SourcePath string `json:"source_path,omitempty"`
}

type MovieUpdateRequest struct {
	LibraryID     string  `json:"library_id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Description   string  `json:"description"`
	Year          int     `json:"year"`
	Genres        string  `json:"genres"`
	Rating        float32 `json:"rating"`
	Runtime       int     `json:"runtime"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	FilePath      string  `json:"file_path"`
}

type TVShow struct {
	ID         string `json:"id"`
	LibraryID  string `json:"library_id"`
	Title      string `json:"title"`
	FolderPath string `json:"folder_path"`
}

type TVShowRequest struct {
	LibraryID  string `json:"library_id"`
	Title      string `json:"title"`
	FolderPath string `json:"folder_path,omitempty"`
}

type Season struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
}

type SeasonRequest struct {
	Number int `json:"number"`
}

type Artist struct {
	ID        string `json:"id"`
	LibraryID string `json:"library_id"`
	Name      string `json:"name"`
}

type ArtistRequest struct {
	LibraryID string `json:"library_id"`
	Name      string `json:"name"`
}

// --- Audio tracks ---

type AudioTrackRequest struct {
	MediaType   string `json:"media_type"`
	MediaID     string `json:"media_id"`
	Language    string `json:"language"`
	Label       string `json:"label"`
	StreamIndex int    `json:"stream_index"`
	FilePath    string `json:"file_path"`
}

type AudioTrack struct {
	ID          string `json:"id"`
	MediaType   string `json:"media_type"`
	MediaID     string `json:"media_id"`
	Language    string `json:"language"`
	Label       string `json:"label"`
	StreamIndex int    `json:"stream_index"`
	FilePath    string `json:"file_path"`
}

// --- Subtitles ---

type SubtitleRequest struct {
	MediaType string `json:"media_type"`
	MediaID   string `json:"media_id"`
	Language  string `json:"language"`
	Label     string `json:"label"`
	FilePath  string `json:"file_path"`
}

type Subtitle struct {
	ID        string `json:"id"`
	MediaType string `json:"media_type"`
	MediaID   string `json:"media_id"`
	Language  string `json:"language"`
	Label     string `json:"label"`
	FilePath  string `json:"file_path"`
}

type Audiobook struct {
	ID        string `json:"id"`
	LibraryID string `json:"library_id"`
	Title     string `json:"title"`
}

type AudiobookRequest struct {
	LibraryID string `json:"library_id"`
	Title     string `json:"title"`
	Year      int    `json:"year,omitempty"`
}

type Client struct {
	baseURL  string
	username string
	password string
	service  string
	token    string
	mu       sync.Mutex
	http     *http.Client
}

func New(baseURL, username, password, service string) *Client {
	return &Client{
		baseURL:  baseURL,
		username: username,
		password: password,
		service:  service,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Log(level, message string) {
	go func() {
		body := map[string]string{"level": level, "service": c.service, "message": message}
		_ = c.do("POST", "/api/logs", body, nil)
	}()
}

func (c *Client) Login() error {
	body, _ := json.Marshal(map[string]string{
		"username": c.username,
		"password": c.password,
	})
	resp, err := c.http.Post(c.baseURL+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login returned status %d", resp.StatusCode)
	}
	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode login response: %w", err)
	}
	c.mu.Lock()
	c.token = result.AccessToken
	c.mu.Unlock()
	return nil
}

func (c *Client) do(method, path string, body, out interface{}) error {
	return c.doWithRetry(method, path, body, out, true)
}

func (c *Client) doWithRetry(method, path string, body, out interface{}, retry bool) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.mu.Lock()
	token := c.token
	c.mu.Unlock()
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized && retry {
		if err := c.Login(); err != nil {
			return fmt.Errorf("re-login: %w", err)
		}
		return c.doWithRetry(method, path, body, out, false)
	}

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %s: status %d: %s", method, path, resp.StatusCode, string(b))
	}

	if out != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func (c *Client) Libraries() ([]Library, error) {
	var libs []Library
	return libs, c.do("GET", "/api/libraries", nil, &libs)
}

// --- Movies ---

func (c *Client) ListMovies(libraryID string) ([]Movie, error) {
	return paginateAll(func(page, limit int) ([]Movie, error) {
		var result []Movie
		path := fmt.Sprintf("/api/movies?library_id=%s&page=%d&limit=%d",
			url.QueryEscape(libraryID), page, limit)
		return result, c.do("GET", path, nil, &result)
	})
}

func (c *Client) CreateMovie(req MovieRequest) (*Movie, error) {
	var result Movie
	return &result, c.do("POST", "/api/movies", req, &result)
}

func (c *Client) GetMovie(id string) (*Movie, error) {
	var result Movie
	return &result, c.do("GET", "/api/movies/"+id, nil, &result)
}

func (c *Client) UpdateMovie(id string, req MovieUpdateRequest) (*Movie, error) {
	var result Movie
	return &result, c.do("PUT", "/api/movies/"+id, req, &result)
}

// UpdateMovieSourcePath targets only source_path so a backfill on a pre-
// existing row can't race-clobber meta-movie's title/poster updates.
func (c *Client) UpdateMovieSourcePath(id, sourcePath string) error {
	body := map[string]string{"source_path": sourcePath}
	return c.do("PATCH", "/api/movies/"+id+"/source-path", body, nil)
}

// --- Audio tracks (used by sidecar registration in scanner) ---

func (c *Client) CreateAudioTrack(req AudioTrackRequest) (*AudioTrack, error) {
	var result AudioTrack
	return &result, c.do("POST", "/api/audio-tracks", req, &result)
}

func (c *Client) ListMovieAudioTracks(movieID string) ([]AudioTrack, error) {
	var result []AudioTrack
	return result, c.do("GET", "/api/movies/"+movieID+"/audio-tracks", nil, &result)
}

func (c *Client) ListEpisodeAudioTracks(showID, seasonID, episodeID string) ([]AudioTrack, error) {
	var result []AudioTrack
	path := "/api/tvshows/" + showID + "/seasons/" + seasonID + "/episodes/" + episodeID + "/audio-tracks"
	return result, c.do("GET", path, nil, &result)
}

// --- Subtitles (used by sidecar registration in scanner) ---

func (c *Client) CreateSubtitle(req SubtitleRequest) (*Subtitle, error) {
	var result Subtitle
	return &result, c.do("POST", "/api/subtitles", req, &result)
}

func (c *Client) ListMovieSubtitles(movieID string) ([]Subtitle, error) {
	var result []Subtitle
	return result, c.do("GET", "/api/movies/"+movieID+"/subtitles", nil, &result)
}

func (c *Client) ListEpisodeSubtitles(showID, seasonID, episodeID string) ([]Subtitle, error) {
	var result []Subtitle
	path := "/api/tvshows/" + showID + "/seasons/" + seasonID + "/episodes/" + episodeID + "/subtitles"
	return result, c.do("GET", path, nil, &result)
}

// --- TV Shows ---

func (c *Client) listTVShows(libraryID string) ([]TVShow, error) {
	return paginateAll(func(page, limit int) ([]TVShow, error) {
		var result []TVShow
		path := fmt.Sprintf("/api/tvshows?library_id=%s&page=%d&limit=%d",
			url.QueryEscape(libraryID), page, limit)
		return result, c.do("GET", path, nil, &result)
	})
}

func (c *Client) createTVShow(req TVShowRequest) (*TVShow, error) {
	var result TVShow
	return &result, c.do("POST", "/api/tvshows", req, &result)
}

func (c *Client) GetTVShow(showID string) (*TVShow, error) {
	var result TVShow
	return &result, c.do("GET", "/api/tvshows/"+showID, nil, &result)
}

// ListTVShows is the exported wrapper around listTVShows. Used by the
// requeue-untranscoded path which has to enumerate every show in a library.
func (c *Client) ListTVShows(libraryID string) ([]TVShow, error) {
	return c.listTVShows(libraryID)
}

// FindOrCreateTVShow looks up a show row that should host the given folder.
//
// Two-pass match because the title alone can't tell distinct series apart
// when they share a name (e.g. the 2005 and 2024 Avatar series both surface
// as "Avatar The Last Airbender"). Merging them onto one row makes the
// admin Identify flow look broken — every periodic scan re-emits both
// folders, meta-tv re-fetches by title, and whichever TMDB returns last
// overwrites the metadata.
//
//   - Pass 1: exact folder_path match wins regardless of title. Survives
//     title renames where the user keeps the same folder on disk.
//   - Pass 2: title match, but only if the candidate has either no
//     folder_path recorded (a legacy row created before folder_path
//     existed) or already equals this one. A row whose folder_path is set
//     to a *different* path is treated as a different show, so the create
//     branch fires below and the two folders end up on separate rows.
func (c *Client) FindOrCreateTVShow(libraryID, title, folderPath string) (*TVShow, error) {
	shows, err := c.listTVShows(libraryID)
	if err != nil {
		return nil, err
	}

	if folderPath != "" {
		for i := range shows {
			if shows[i].FolderPath == folderPath {
				return &shows[i], nil
			}
		}
	}

	needle := matchKey(title)
	for i := range shows {
		if matchKey(shows[i].Title) != needle {
			continue
		}
		if shows[i].FolderPath == "" || shows[i].FolderPath == folderPath {
			return &shows[i], nil
		}
	}

	return c.createTVShow(TVShowRequest{LibraryID: libraryID, Title: title, FolderPath: folderPath})
}

// UpdateTVShowFolderPath targets only the show's folder_path field, so the
// scanner can record where on disk a show lives without rewriting any
// metadata that meta-tv has populated since.
func (c *Client) UpdateTVShowFolderPath(showID, folderPath string) error {
	body := map[string]string{"folder_path": folderPath}
	return c.do("PATCH", "/api/tvshows/"+showID+"/folder-path", body, nil)
}

// --- Seasons ---

func (c *Client) listSeasons(showID string) ([]Season, error) {
	var result []Season
	return result, c.do("GET", "/api/tvshows/"+showID+"/seasons", nil, &result)
}

func (c *Client) createSeason(showID string, req SeasonRequest) (*Season, error) {
	var result Season
	return &result, c.do("POST", "/api/tvshows/"+showID+"/seasons", req, &result)
}

// ListSeasons is the exported wrapper around listSeasons. Used by the
// requeue-untranscoded path which walks every season of every show.
func (c *Client) ListSeasons(showID string) ([]Season, error) {
	return c.listSeasons(showID)
}

func (c *Client) FindOrCreateSeason(showID string, number int) (*Season, error) {
	seasons, err := c.listSeasons(showID)
	if err != nil {
		return nil, err
	}
	for i := range seasons {
		if seasons[i].Number == number {
			return &seasons[i], nil
		}
	}
	return c.createSeason(showID, SeasonRequest{Number: number})
}

// --- Episodes ---

type Episode struct {
	ID         string `json:"id"`
	Number     int    `json:"number"`
	Title      string `json:"title"`
	FilePath   string `json:"file_path"`
	SourcePath string `json:"source_path"`
}

type EpisodeRequest struct {
	Number     int    `json:"number"`
	FilePath   string `json:"file_path"`
	SourcePath string `json:"source_path,omitempty"`
}

func (c *Client) ListEpisodes(showID, seasonID string) ([]Episode, error) {
	var result []Episode
	path := fmt.Sprintf("/api/tvshows/%s/seasons/%s/episodes", showID, seasonID)
	return result, c.do("GET", path, nil, &result)
}

func (c *Client) CreateEpisode(showID, seasonID string, req EpisodeRequest) (*Episode, error) {
	var result Episode
	path := fmt.Sprintf("/api/tvshows/%s/seasons/%s/episodes", showID, seasonID)
	return &result, c.do("POST", path, req, &result)
}

func (c *Client) UpdateEpisode(showID, seasonID, episodeID string, req EpisodeRequest) (*Episode, error) {
	var result Episode
	path := fmt.Sprintf("/api/tvshows/%s/seasons/%s/episodes/%s", showID, seasonID, episodeID)
	return &result, c.do("PUT", path, req, &result)
}

// UpdateEpisodeSourcePath targets only source_path. Used to backfill the
// original on-disk location on rows that were created before the field
// existed (or via a code path that didn't populate it) — needed for the
// stream-endpoint fallback to find a file when the transcoded one is gone.
func (c *Client) UpdateEpisodeSourcePath(showID, seasonID, episodeID, sourcePath string) error {
	body := map[string]string{"source_path": sourcePath}
	path := fmt.Sprintf("/api/tvshows/%s/seasons/%s/episodes/%s/source-path", showID, seasonID, episodeID)
	return c.do("PATCH", path, body, nil)
}

// --- Artists ---

func (c *Client) listArtists(libraryID string) ([]Artist, error) {
	return paginateAll(func(page, limit int) ([]Artist, error) {
		var result []Artist
		path := fmt.Sprintf("/api/artists?library_id=%s&page=%d&limit=%d",
			url.QueryEscape(libraryID), page, limit)
		return result, c.do("GET", path, nil, &result)
	})
}

func (c *Client) createArtist(req ArtistRequest) (*Artist, error) {
	var result Artist
	return &result, c.do("POST", "/api/artists", req, &result)
}

func (c *Client) FindOrCreateArtist(libraryID, name string) (*Artist, error) {
	artists, err := c.listArtists(libraryID)
	if err != nil {
		return nil, err
	}
	needle := matchKey(name)
	for i := range artists {
		if matchKey(artists[i].Name) == needle {
			return &artists[i], nil
		}
	}
	return c.createArtist(ArtistRequest{LibraryID: libraryID, Name: name})
}

// --- Albums ---

type Album struct {
	ID        string `json:"id"`
	LibraryID string `json:"library_id"`
	ArtistID  string `json:"artist_id"`
	Title     string `json:"title"`
	Year      int    `json:"year"`
}

type AlbumRequest struct {
	LibraryID string `json:"library_id"`
	ArtistID  string `json:"artist_id,omitempty"`
	Title     string `json:"title"`
	Year      int    `json:"year,omitempty"`
}

func (c *Client) listAlbums(libraryID string) ([]Album, error) {
	return paginateAll(func(page, limit int) ([]Album, error) {
		var result []Album
		path := fmt.Sprintf("/api/albums?library_id=%s&page=%d&limit=%d",
			url.QueryEscape(libraryID), page, limit)
		return result, c.do("GET", path, nil, &result)
	})
}

func (c *Client) createAlbum(req AlbumRequest) (*Album, error) {
	var result Album
	return &result, c.do("POST", "/api/albums", req, &result)
}

func (c *Client) FindOrCreateAlbum(libraryID, artistID, title string, year int) (*Album, error) {
	albums, err := c.listAlbums(libraryID)
	if err != nil {
		return nil, err
	}
	needle := matchKey(title)
	for i := range albums {
		if matchKey(albums[i].Title) == needle && albums[i].ArtistID == artistID {
			return &albums[i], nil
		}
	}
	return c.createAlbum(AlbumRequest{LibraryID: libraryID, ArtistID: artistID, Title: title, Year: year})
}

// --- Tracks ---

type Track struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
}

type TrackRequest struct {
	LibraryID string `json:"library_id"`
	AlbumID   string `json:"album_id"`
	ArtistID  string `json:"artist_id,omitempty"`
	Title     string `json:"title"`
	Number    int    `json:"number,omitempty"`
	FilePath  string `json:"file_path"`
}

func (c *Client) ListAlbumTracks(albumID string) ([]Track, error) {
	return paginateAll(func(page, limit int) ([]Track, error) {
		var result []Track
		path := fmt.Sprintf("/api/albums/%s/tracks?page=%d&limit=%d", albumID, page, limit)
		return result, c.do("GET", path, nil, &result)
	})
}

func (c *Client) CreateTrack(req TrackRequest) (*Track, error) {
	var result Track
	return &result, c.do("POST", "/api/tracks", req, &result)
}

// --- Audiobooks ---

func (c *Client) listAudiobooks(libraryID string) ([]Audiobook, error) {
	return paginateAll(func(page, limit int) ([]Audiobook, error) {
		var result []Audiobook
		path := fmt.Sprintf("/api/audiobooks?library_id=%s&page=%d&limit=%d",
			url.QueryEscape(libraryID), page, limit)
		return result, c.do("GET", path, nil, &result)
	})
}

func (c *Client) createAudiobook(req AudiobookRequest) (*Audiobook, error) {
	var result Audiobook
	return &result, c.do("POST", "/api/audiobooks", req, &result)
}

func (c *Client) FindOrCreateAudiobook(libraryID, title string, year int) (*Audiobook, error) {
	books, err := c.listAudiobooks(libraryID)
	if err != nil {
		return nil, err
	}
	needle := matchKey(title)
	for i := range books {
		if matchKey(books[i].Title) == needle {
			return &books[i], nil
		}
	}
	return c.createAudiobook(AudiobookRequest{LibraryID: libraryID, Title: title, Year: year})
}

// --- Chapters ---

type Chapter struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
}

type ChapterRequest struct {
	Number   int    `json:"number"`
	Title    string `json:"title,omitempty"`
	Duration int    `json:"duration,omitempty"`
	FilePath string `json:"file_path"`
}

func (c *Client) ListChapters(audiobookID string) ([]Chapter, error) {
	var result []Chapter
	return result, c.do("GET", "/api/audiobooks/"+audiobookID+"/chapters", nil, &result)
}

func (c *Client) CreateChapter(audiobookID string, req ChapterRequest) (*Chapter, error) {
	var result Chapter
	return &result, c.do("POST", "/api/audiobooks/"+audiobookID+"/chapters", req, &result)
}

// ---

func paginateAll[T any](fetch func(page, limit int) ([]T, error)) ([]T, error) {
	const limit = 200
	var all []T
	for page := 1; ; page++ {
		items, err := fetch(page, limit)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) < limit {
			break
		}
	}
	return all, nil
}
