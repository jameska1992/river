package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"river-meta-tv/internal/apiclient"
	"river-meta-tv/internal/consumer"
	"river-meta-tv/internal/nameinfo"
	tmdbpkg "river-meta-tv/internal/tmdb"
)

// SxxExx e.g. S01E03, s1e3
var episodePattern = regexp.MustCompile(`(?i)[Ss]\d{1,2}[Ee](\d{1,3})`)

// NxNN e.g. 1x07, 23x14 — the Plex / DVD-rip naming style. The word
// boundaries are important: without them this would also match the
// trailing digits of strings like "x265" or "H.264".
var episodeXPattern = regexp.MustCompile(`\b\d{1,2}x(\d{1,3})\b`)

// Fallback: plain E/e number e.g. E03, e3
var episodeFallback = regexp.MustCompile(`(?i)[Ee](\d{1,3})`)

// Extracts the first plain number from a string (e.g. "Season 2" → 2)
var numberPattern = regexp.MustCompile(`\d+`)

type Processor struct {
	api  *apiclient.Client
	tmdb *tmdbpkg.Client
}

func New(api *apiclient.Client, tmdb *tmdbpkg.Client) *Processor {
	return &Processor{api: api, tmdb: tmdb}
}

func (p *Processor) Handle(event consumer.MediaDiscoveredEvent) error {
	seasonNumber := parseSeasonNumber(event.SeasonName)

	var show *apiclient.TVShow
	if event.MediaID != "" {
		s, err := p.api.GetShow(event.MediaID)
		if err != nil {
			return fmt.Errorf("get show %s: %w", event.MediaID, err)
		}
		show = s
	} else {
		// Fallback: scanner didn't tag the event with a show id (legacy queue
		// entry, or pre-existing data). Normalize the folder name with the
		// PTN-style parser so release-named folders still match an existing
		// record by title.
		parsed := nameinfo.Parse(event.DirectoryName)
		needle := parsed.Title
		if needle == "" {
			needle = event.DirectoryName
		}
		shows, err := p.api.ListShows(event.LibraryID)
		if err != nil {
			return fmt.Errorf("list shows: %w", err)
		}
		show = findShow(shows, needle, parsed.Year)
		if show == nil {
			log.Printf("INFO show %q (year=%d) not found in library %s, skipping", needle, parsed.Year, event.LibraryID)
			return nil
		}
	}

	meta, tmdbShowID, err := p.enrichShow(show, event.TMDBID, event.IMDBID)
	if err != nil {
		return err
	}
	if tmdbShowID == 0 {
		return nil // not found on TMDB, already logged
	}

	season, err := p.syncSeason(show.ID, seasonNumber, tmdbShowID, event.SeasonID)
	if err != nil {
		return fmt.Errorf("sync season %d for show %s: %w", seasonNumber, show.ID, err)
	}

	if err := p.syncEpisodes(show.ID, season, tmdbShowID, seasonNumber, event.Files); err != nil {
		return fmt.Errorf("sync episodes for season %d show %s: %w", seasonNumber, show.ID, err)
	}

	_ = meta
	return nil
}

// RefreshByID re-fetches TMDB metadata for a show and all its existing episodes.
func (p *Processor) RefreshByID(showID string) error {
	return p.RefreshByIDWithIMDB(showID, "")
}

// RefreshByIDWithIMDB enriches a show, optionally biasing the TMDB lookup
// with an explicit IMDb id. Used by the admin "identify" flow when the
// on-disk show folder doesn't match TMDB cleanly; the title/year override
// (if any) is persisted on the river-api side before this is called.
func (p *Processor) RefreshByIDWithIMDB(showID, imdbID string) error {
	show, err := p.api.GetShow(showID)
	if err != nil {
		return fmt.Errorf("get show %s: %w", showID, err)
	}

	meta, tmdbShowID, err := p.enrichShow(show, 0, imdbID)
	if err != nil {
		return err
	}
	if tmdbShowID == 0 {
		return nil
	}
	_ = meta

	seasons, err := p.api.ListSeasons(showID)
	if err != nil {
		return fmt.Errorf("list seasons: %w", err)
	}

	for _, season := range seasons {
		seasonMeta, err := p.tmdb.FetchSeasonMetadata(tmdbShowID, season.Number)
		if err != nil {
			log.Printf("WARN tmdb season %d unavailable for show %s: %v", season.Number, showID, err)
			continue
		}

		tmdbByNumber := make(map[int]tmdbpkg.EpisodeMetadata, len(seasonMeta.Episodes))
		for _, e := range seasonMeta.Episodes {
			tmdbByNumber[e.Number] = e
		}

		episodes, err := p.api.ListEpisodes(showID, season.ID)
		if err != nil {
			log.Printf("WARN list episodes for season %s: %v", season.ID, err)
			continue
		}

		for _, ep := range episodes {
			epMeta, ok := tmdbByNumber[ep.Number]
			if !ok {
				continue
			}
			airedAt := ""
			if epMeta.AiredAt != "" {
				airedAt = epMeta.AiredAt + "T00:00:00Z"
			}
			if _, err := p.api.UpdateEpisode(showID, season.ID, ep.ID, apiclient.EpisodeRequest{
				Title:       epMeta.Title,
				Description: epMeta.Description,
				Runtime:     epMeta.Runtime,
				AiredAt:     airedAt,
			}); err != nil {
				log.Printf("WARN update episode %d in season %s: %v", ep.Number, season.ID, err)
			}
		}
		log.Printf("INFO refreshed %d episodes in season %d for show %s", len(episodes), season.Number, showID)
	}

	return nil
}

// enrichShow updates the show record in river-api with TMDB data and returns the TMDB show ID.
// Returns tmdbShowID=0 if the show is not found on TMDB (already logged, not an error).
// hintTMDB/hintIMDB come from the scanner (embedded {tmdb-N}/{imdb-ttN} tags,
// tvshow.nfo sidecar) and skip the title-similarity search when present.
func (p *Processor) enrichShow(show *apiclient.TVShow, hintTMDB int, hintIMDB string) (*tmdbpkg.ShowMetadata, int, error) {
	meta, err := p.fetchShow(show, hintTMDB, hintIMDB)
	if err != nil {
		if errors.Is(err, tmdbpkg.ErrNotFound) {
			log.Printf("WARN tmdb: no results for show %q, skipping enrichment", show.Title)
			p.api.Log("warn", fmt.Sprintf("failed to identify tvshow %q: no TMDB match", show.Title))
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("tmdb fetch show %q: %w", show.Title, err)
	}

	genresJSON, err := json.Marshal(meta.Genres)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal genres: %w", err)
	}

	// Prefer the TMDB-canonical title over the dir-parsed one — folder names
	// are noisier than what TMDB returns ("The.Last.of.Us.S01" vs "The Last of Us").
	title := show.Title
	if meta.Title != "" {
		title = meta.Title
	}
	if _, err := p.api.UpdateShow(show.ID, apiclient.TVShowRequest{
		LibraryID:     show.LibraryID,
		Title:         title,
		OriginalTitle: meta.OriginalTitle,
		Description:   meta.Description,
		Year:          meta.Year,
		Status:        meta.Status,
		Genres:        string(genresJSON),
		Rating:        meta.Rating,
		PosterPath:    meta.PosterURL,
		BackdropPath:  meta.BackdropURL,
		TrailerURL:    meta.TrailerURL,
		// Persist the resolved TMDB id so subsequent enrichments (rescan
		// events, future refresh calls) go straight to this show rather
		// than re-running a title search that could pick a different
		// popular candidate. This is what makes an admin "identify via
		// IMDb" actually stick across refresh.
		TMDBID: meta.TMDBShowID,
	}); err != nil {
		return nil, 0, fmt.Errorf("update show %s: %w", show.ID, err)
	}
	log.Printf("INFO enriched show %q (id=%s) from TMDB", show.Title, show.ID)
	yearTag := ""
	if meta.Year > 0 {
		yearTag = fmt.Sprintf(" (%d)", meta.Year)
	}
	p.api.Log("info", fmt.Sprintf("identified tvshow %q%s via TMDB", show.Title, yearTag))

	cast := make([]apiclient.CastCredit, len(meta.Cast))
	for i, c := range meta.Cast {
		cast[i] = apiclient.CastCredit{TmdbID: c.TmdbID, Name: c.Name, ProfilePath: c.ProfilePath, Biography: c.Biography, Character: c.Character, Order: c.Order}
	}
	crew := make([]apiclient.CrewCredit, len(meta.Crew))
	for i, c := range meta.Crew {
		crew[i] = apiclient.CrewCredit{TmdbID: c.TmdbID, Name: c.Name, ProfilePath: c.ProfilePath, Biography: c.Biography, Job: c.Job, Department: c.Department}
	}
	if err := p.api.SetTVShowCredits(show.ID, apiclient.CreditsRequest{Cast: cast, Crew: crew}); err != nil {
		log.Printf("WARN failed to set credits for show %s: %v", show.ID, err)
	}

	return meta, meta.TMDBShowID, nil
}

// fetchShow picks the most accurate TMDB lookup strategy: TMDB id is a
// direct hit, IMDb id resolves via /find, otherwise we fall back to the
// scored title search. An IMDb id that 404s falls through to title search
// so a stale tag doesn't block enrichment. When the title search itself
// returns no match, nameinfo.RetryTitles produces cleanup variants
// (strip "remake", strip trailing " 1" runs) and we try each.
func (p *Processor) fetchShow(show *apiclient.TVShow, hintTMDB int, hintIMDB string) (*tmdbpkg.ShowMetadata, error) {
	if hintTMDB > 0 {
		log.Printf("INFO tmdb: using TMDB ID %d for show %q", hintTMDB, show.Title)
		return p.tmdb.FetchShowByTMDBID(hintTMDB)
	}
	if hintIMDB != "" {
		log.Printf("INFO tmdb: using IMDb ID %s for show %q", hintIMDB, show.Title)
		meta, err := p.tmdb.FetchShowByIMDBID(hintIMDB)
		if err == nil {
			return meta, nil
		}
		if !errors.Is(err, tmdbpkg.ErrNotFound) {
			return nil, err
		}
		log.Printf("WARN tmdb: IMDb ID %s not found, falling back to title search", hintIMDB)
	}
	// Stored TMDB id wins over title search. Once the show has been
	// successfully resolved once — via title match, IMDb hint, or admin
	// override — we use that same id for every future enrichment so a
	// rescan can't drift to a different popular candidate.
	if show.TMDBID > 0 {
		log.Printf("INFO tmdb: using stored TMDB ID %d for show %q", show.TMDBID, show.Title)
		return p.tmdb.FetchShowByTMDBID(show.TMDBID)
	}

	meta, err := p.tmdb.FetchShowMetadata(show.Title, show.Year)
	if err == nil || !errors.Is(err, tmdbpkg.ErrNotFound) {
		return meta, err
	}

	for _, alt := range nameinfo.RetryTitles(show.Title) {
		log.Printf("INFO tmdb: retrying show %q as %q", show.Title, alt)
		meta, retryErr := p.tmdb.FetchShowMetadata(alt, show.Year)
		if retryErr == nil {
			return meta, nil
		}
		if !errors.Is(retryErr, tmdbpkg.ErrNotFound) {
			return nil, retryErr
		}
	}
	return nil, err
}

// syncSeason ensures the season record exists and returns it.
// If knownSeasonID is provided (from the event), it fetches that season directly.
// Otherwise it lists seasons and creates one if missing.
func (p *Processor) syncSeason(showID string, seasonNumber, tmdbShowID int, knownSeasonID string) (*apiclient.Season, error) {
	seasons, err := p.api.ListSeasons(showID)
	if err != nil {
		return nil, fmt.Errorf("list seasons: %w", err)
	}

	for i := range seasons {
		if seasons[i].Number == seasonNumber {
			return &seasons[i], nil
		}
		if knownSeasonID != "" && seasons[i].ID == knownSeasonID {
			return &seasons[i], nil
		}
	}

	// Season does not exist yet — fetch metadata and create it.
	seasonMeta, err := p.tmdb.FetchSeasonMetadata(tmdbShowID, seasonNumber)
	if err != nil {
		// Non-fatal: create a minimal season record if TMDB has no data for this season.
		log.Printf("WARN tmdb season %d not found for tmdb show %d: %v", seasonNumber, tmdbShowID, err)
		seasonMeta = &tmdbpkg.SeasonMetadata{Number: seasonNumber}
	}

	created, err := p.api.CreateSeason(showID, apiclient.SeasonRequest{
		Number:      seasonNumber,
		Title:       seasonMeta.Title,
		Description: seasonMeta.Description,
		Year:        seasonMeta.Year,
		PosterPath:  seasonMeta.PosterURL,
	})
	if err != nil {
		return nil, fmt.Errorf("create season %d: %w", seasonNumber, err)
	}
	log.Printf("INFO created season %d for show %s", seasonNumber, showID)
	return created, nil
}

func (p *Processor) syncEpisodes(showID string, season *apiclient.Season, tmdbShowID, seasonNumber int, files []string) error {
	existing, err := p.api.ListEpisodes(showID, season.ID)
	if err != nil {
		return fmt.Errorf("list episodes: %w", err)
	}

	// Regular and special episodes share a Number namespace but are
	// distinguished by IsSpecial, so the existing-record lookup has to key
	// on both.
	type epKey struct {
		number    int
		isSpecial bool
	}
	existingByKey := make(map[epKey]string, len(existing))
	specialsBySource := make(map[string]string, len(existing)) // source_path → ID
	maxSpecialNum := 0
	for _, e := range existing {
		existingByKey[epKey{e.Number, e.IsSpecial}] = e.ID
		if e.IsSpecial {
			// Match by source_path so we don't duplicate on re-scan even if
			// video-trans hasn't populated file_path yet.
			if e.SourcePath != "" {
				specialsBySource[e.SourcePath] = e.ID
			}
			if e.Number > maxSpecialNum {
				maxSpecialNum = e.Number
			}
		}
	}

	// Fetch TMDB episode index for this season to get metadata per episode number.
	seasonMeta, err := p.tmdb.FetchSeasonMetadata(tmdbShowID, seasonNumber)
	if err != nil {
		log.Printf("WARN tmdb episode metadata unavailable for season %d: %v", seasonNumber, err)
		seasonMeta = &tmdbpkg.SeasonMetadata{}
	}
	tmdbByNumber := make(map[int]tmdbpkg.EpisodeMetadata, len(seasonMeta.Episodes))
	for _, e := range seasonMeta.Episodes {
		tmdbByNumber[e.Number] = e
	}

	// Split into regular vs special so specials get stable 1..N sequence
	// numbers ordered by filename (we don't know which "Behind the Scenes"
	// file is the first one from TMDB's perspective).
	var regularFiles []string
	var specialFiles []string
	for _, filePath := range files {
		if parseEpisodeNumber(filepath.Base(filePath)) == 0 {
			specialFiles = append(specialFiles, filePath)
		} else {
			regularFiles = append(regularFiles, filePath)
		}
	}
	sort.Strings(specialFiles)

	for _, filePath := range regularFiles {
		epNum := parseEpisodeNumber(filepath.Base(filePath))
		epMeta := tmdbByNumber[epNum]
		airedAt := ""
		if epMeta.AiredAt != "" {
			airedAt = epMeta.AiredAt + "T00:00:00Z"
		}

		if epID, found := existingByKey[epKey{epNum, false}]; found {
			// Episode already exists — update metadata only, leave file_path untouched.
			_, err := p.api.UpdateEpisode(showID, season.ID, epID, apiclient.EpisodeRequest{
				Title:       epMeta.Title,
				Description: epMeta.Description,
				Runtime:     epMeta.Runtime,
				AiredAt:     airedAt,
			})
			if err != nil {
				log.Printf("ERROR update episode %d metadata for season %s: %v", epNum, season.ID, err)
			} else {
				log.Printf("INFO updated episode %d metadata in season %s", epNum, season.ID)
			}
			continue
		}

		ep, err := p.api.CreateEpisode(showID, season.ID, apiclient.EpisodeRequest{
			Number:      epNum,
			Title:       epMeta.Title,
			Description: epMeta.Description,
			Runtime:     epMeta.Runtime,
			AiredAt:     airedAt,
		})
		if err != nil {
			log.Printf("ERROR create episode %d for season %s: %v", epNum, season.ID, err)
			continue
		}
		log.Printf("INFO created episode %d (id=%s) in season %s", epNum, ep.ID, season.ID)
		existingByKey[epKey{epNum, false}] = ep.ID
	}

	// Specials — no parseable number. Skip ones already known (matched by
	// file_path); the rest get the next available sequence number above
	// the highest existing special, so a re-scan after the user adds
	// another bonus file doesn't shuffle the assignment of existing rows.
	for _, filePath := range specialFiles {
		if _, found := specialsBySource[filePath]; found {
			continue
		}
		maxSpecialNum++
		seq := maxSpecialNum
		title := specialTitleFromFilename(filePath)
		ep, err := p.api.CreateEpisode(showID, season.ID, apiclient.EpisodeRequest{
			Number:     seq,
			Title:      title,
			SourcePath: filePath,
			IsSpecial:  true,
		})
		if err != nil {
			log.Printf("ERROR create special %d (%q) for season %s: %v", seq, filePath, season.ID, err)
			maxSpecialNum--
			continue
		}
		log.Printf("INFO created special %d (id=%s, %q) in season %s", seq, ep.ID, filePath, season.ID)
		existingByKey[epKey{seq, true}] = ep.ID
		specialsBySource[filePath] = ep.ID
	}

	return nil
}

// specialTitleFromFilename derives a display title for a special episode
// when we have no TMDB metadata to fall back on. Strips the extension and
// replaces separator characters so "Behind.the.Scenes.mkv" reads as
// "Behind the Scenes" in the UI.
func specialTitleFromFilename(p string) string {
	base := filepath.Base(p)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	stem = strings.NewReplacer(".", " ", "_", " ").Replace(stem)
	return strings.TrimSpace(stem)
}

func parseSeasonNumber(seasonName string) int {
	m := numberPattern.FindString(seasonName)
	if m == "" {
		return 0
	}
	n, _ := strconv.Atoi(m)
	return n
}

func parseEpisodeNumber(filename string) int {
	// Order matters: SxxExx is the most specific, NxNN is next, and the
	// bare E\d+ fallback comes last because it can easily false-match
	// (e.g. "HEVC" / "H.264" sequences).
	if m := episodePattern.FindStringSubmatch(filename); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	if m := episodeXPattern.FindStringSubmatch(filename); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	if m := episodeFallback.FindStringSubmatch(filename); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

// findShow resolves the library record for an incoming show event. Title
// is compared case-insensitively. Year disambiguates: when both the
// event and a candidate have a year set, they must match exactly —
// otherwise two shows sharing a title (Doctor Who 1963 vs 2005,
// Battlestar Galactica 1978 vs 2004) would collapse onto whichever
// record was created first. A title-only fallback is preserved for the
// case where neither side has a year yet.
func findShow(shows []apiclient.TVShow, name string, year int) *apiclient.TVShow {
	needle := strings.ToLower(strings.TrimSpace(name))
	var fallback *apiclient.TVShow
	for i := range shows {
		if strings.ToLower(strings.TrimSpace(shows[i].Title)) != needle {
			continue
		}
		if year > 0 && shows[i].Year == year {
			return &shows[i]
		}
		if (year == 0 || shows[i].Year == 0) && fallback == nil {
			fallback = &shows[i]
		}
	}
	return fallback
}
