package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"river-meta-movie/internal/apiclient"
	"river-meta-movie/internal/consumer"
	"river-meta-movie/internal/nameinfo"
	tmdbpkg "river-meta-movie/internal/tmdb"
)

type Processor struct {
	api  *apiclient.Client
	tmdb *tmdbpkg.Client
}

func New(api *apiclient.Client, tmdb *tmdbpkg.Client) *Processor {
	return &Processor{api: api, tmdb: tmdb}
}

func (p *Processor) Handle(event consumer.MediaDiscoveredEvent) error {
	var movie *apiclient.Movie
	if event.MediaID != "" {
		m, err := p.api.GetMovie(event.MediaID)
		if err != nil {
			return fmt.Errorf("get movie %s: %w", event.MediaID, err)
		}
		movie = m
	} else {
		info := nameinfo.Parse(event.DirectoryName)
		movies, err := p.api.ListMovies(event.LibraryID)
		if err != nil {
			return fmt.Errorf("list movies: %w", err)
		}
		movie = findMovie(movies, info.Title, info.Year)
		if movie == nil {
			log.Printf("INFO movie %q (year=%d) not found in library %s, skipping", info.Title, info.Year, event.LibraryID)
			return nil
		}
	}
	return p.enrich(movie, event.TMDBID, event.IMDBID, parentDirHint(event))
}

// parentDirHint returns the containing-directory name when it's not the
// library root. This is an extra TMDB search query to try when the file-
// based title fails to match — common when the filename is release-named
// ("inception.1080p.bluray.mkv") but the folder is curated ("Inception
// (2010)/"). Returns "" when the parent IS the library root (no curated
// context above a flat file) or when path info is missing from the event.
func parentDirHint(event consumer.MediaDiscoveredEvent) string {
	if event.DirectoryPath == "" || event.LibraryPath == "" {
		return ""
	}
	if filepath.Clean(event.DirectoryPath) == filepath.Clean(event.LibraryPath) {
		return ""
	}
	return nameinfo.Parse(event.DirectoryName).Title
}

func (p *Processor) RefreshByID(movieID string) error {
	return p.RefreshByIDWithIMDB(movieID, "")
}

// RefreshByIDWithIMDB enriches a movie, optionally biasing the TMDB lookup
// with an explicit IMDb id. Used by the admin "identify" flow — when a
// movie's on-disk title is too garbled to match cleanly, an admin can paste
// the IMDb id and we resolve via TMDB's /find endpoint, sidestepping the
// title-similarity search. Title/year overrides are persisted on the river-
// api side before this is called, so the movie record passed to enrich
// already reflects the admin's chosen title/year.
func (p *Processor) RefreshByIDWithIMDB(movieID, imdbID string) error {
	movie, err := p.api.GetMovie(movieID)
	if err != nil {
		return fmt.Errorf("get movie %s: %w", movieID, err)
	}
	// Admin refresh has no event context; parent-dir hint is N/A.
	return p.enrich(movie, 0, imdbID, "")
}

// enrich fetches TMDB metadata for movie and writes it back to river-api.
// hintTMDB/hintIMDB are external IDs extracted by the scanner (from folder
// tags or NFO sidecars); when present they let us skip title-based search.
// parentHint is the containing directory's parsed name (empty when the
// containing directory IS the library root) and gets tried as an extra
// TMDB search query after the primary lookup and RetryTitles all miss.
func (p *Processor) enrich(movie *apiclient.Movie, hintTMDB int, hintIMDB, parentHint string) error {
	meta, err := p.fetch(movie, hintTMDB, hintIMDB, parentHint)
	if err != nil {
		if errors.Is(err, tmdbpkg.ErrNotFound) {
			log.Printf("WARN tmdb: no results for %q, skipping enrichment", movie.Title)
			p.api.Log("warn", fmt.Sprintf("failed to identify movie %q (year=%d): no TMDB match", movie.Title, movie.Year))
			return nil
		}
		return fmt.Errorf("tmdb fetch %q: %w", movie.Title, err)
	}

	genresJSON, err := json.Marshal(meta.Genres)
	if err != nil {
		return fmt.Errorf("marshal genres: %w", err)
	}

	// Prefer the TMDB-canonical title over the dir-parsed one we created the
	// record with — file/folder names are usually noisier ("star.wars.iv.1977")
	// than what TMDB returns ("Star Wars: Episode IV – A New Hope").
	title := movie.Title
	if meta.Title != "" {
		title = meta.Title
	}
	req := apiclient.MovieRequest{
		LibraryID:     movie.LibraryID,
		Title:         title,
		OriginalTitle: meta.OriginalTitle,
		Description:   meta.Description,
		Year:          meta.Year,
		Genres:        string(genresJSON),
		Rating:        meta.Rating,
		Runtime:       meta.Runtime,
		PosterPath:    meta.PosterURL,
		BackdropPath:  meta.BackdropURL,
		TrailerURL:    meta.TrailerURL,
		FilePath:      movie.FilePath,
		// Persist the resolved TMDB id so subsequent enrichments (rescan
		// events, future refresh calls) go straight to this movie rather
		// than re-running a title+year search that could pick a different
		// popular candidate. This is what makes an admin "identify via
		// IMDb" actually stick across refresh.
		TMDBID: meta.TMDBID,
	}

	if _, err := p.api.UpdateMovie(movie.ID, req); err != nil {
		return fmt.Errorf("update movie %s: %w", movie.ID, err)
	}

	cast := make([]apiclient.CastCredit, len(meta.Cast))
	for i, c := range meta.Cast {
		cast[i] = apiclient.CastCredit{TmdbID: c.TmdbID, Name: c.Name, ProfilePath: c.ProfilePath, Biography: c.Biography, Character: c.Character, Order: c.Order}
	}
	crew := make([]apiclient.CrewCredit, len(meta.Crew))
	for i, c := range meta.Crew {
		crew[i] = apiclient.CrewCredit{TmdbID: c.TmdbID, Name: c.Name, ProfilePath: c.ProfilePath, Biography: c.Biography, Job: c.Job, Department: c.Department}
	}
	if err := p.api.SetMovieCredits(movie.ID, apiclient.CreditsRequest{Cast: cast, Crew: crew}); err != nil {
		log.Printf("WARN failed to set credits for movie %s: %v", movie.ID, err)
	}

	log.Printf("INFO enriched movie %q (id=%s) from TMDB", movie.Title, movie.ID)
	yearTag := ""
	if meta.Year > 0 {
		yearTag = fmt.Sprintf(" (%d)", meta.Year)
	}
	p.api.Log("info", fmt.Sprintf("identified movie %q%s via TMDB", meta.Title, yearTag))
	return nil
}

// fetch picks the most accurate TMDB lookup strategy: TMDB ID is a direct
// hit; IMDb ID resolves via /find; otherwise we fall back to title+year
// search. IMDb lookups that 404 fall through to the title search so a stale
// or invalid ID doesn't block enrichment entirely. If the title search
// itself returns no match, nameinfo.RetryTitles produces cleanup variants
// (strip "remake", strip trailing " 1" runs) and we try each. As a final
// fallback we try parentHint — the containing directory's parsed name —
// since folder names are typically curated even when filenames aren't.
func (p *Processor) fetch(movie *apiclient.Movie, hintTMDB int, hintIMDB, parentHint string) (*tmdbpkg.Metadata, error) {
	if hintTMDB > 0 {
		log.Printf("INFO tmdb: using TMDB ID %d for %q", hintTMDB, movie.Title)
		return p.tmdb.FetchByTMDBID(hintTMDB)
	}
	if hintIMDB != "" {
		log.Printf("INFO tmdb: using IMDb ID %s for %q", hintIMDB, movie.Title)
		meta, err := p.tmdb.FetchByIMDBID(hintIMDB)
		if err == nil {
			return meta, nil
		}
		if !errors.Is(err, tmdbpkg.ErrNotFound) {
			return nil, err
		}
		log.Printf("WARN tmdb: IMDb ID %s not found, falling back to title search", hintIMDB)
	}
	// Stored TMDB id wins over title search. Once the movie has been
	// successfully resolved once — via title+year match, IMDb hint, or
	// admin override — we use that same id for every future enrichment so
	// a rescan can't drift to a different popular candidate.
	if movie.TMDBID > 0 {
		log.Printf("INFO tmdb: using stored TMDB ID %d for %q", movie.TMDBID, movie.Title)
		return p.tmdb.FetchByTMDBID(movie.TMDBID)
	}

	meta, err := p.tmdb.FetchMetadata(movie.Title, movie.Year)
	if err == nil || !errors.Is(err, tmdbpkg.ErrNotFound) {
		return meta, err
	}

	for _, alt := range nameinfo.RetryTitles(movie.Title) {
		log.Printf("INFO tmdb: retrying %q as %q", movie.Title, alt)
		meta, retryErr := p.tmdb.FetchMetadata(alt, movie.Year)
		if retryErr == nil {
			return meta, nil
		}
		if !errors.Is(retryErr, tmdbpkg.ErrNotFound) {
			return nil, retryErr
		}
	}

	// Containing directory as a final hint (skipped when it's the library
	// root — parentDirHint already filters those out). Try the parent name
	// directly, then its RetryTitles variants.
	if parentHint != "" && !strings.EqualFold(strings.TrimSpace(parentHint), strings.TrimSpace(movie.Title)) {
		log.Printf("INFO tmdb: retrying %q with parent dir hint %q", movie.Title, parentHint)
		meta, retryErr := p.tmdb.FetchMetadata(parentHint, movie.Year)
		if retryErr == nil {
			return meta, nil
		}
		if !errors.Is(retryErr, tmdbpkg.ErrNotFound) {
			return nil, retryErr
		}
		for _, alt := range nameinfo.RetryTitles(parentHint) {
			log.Printf("INFO tmdb: retrying parent hint %q as %q", parentHint, alt)
			meta, retryErr := p.tmdb.FetchMetadata(alt, movie.Year)
			if retryErr == nil {
				return meta, nil
			}
			if !errors.Is(retryErr, tmdbpkg.ErrNotFound) {
				return nil, retryErr
			}
		}
	}

	return nil, err
}

// findMovie resolves the library record for an incoming event. Title is
// compared case-insensitively. Year is the disambiguator: when both the
// event and a candidate record have a year set, they must match exactly —
// otherwise two different movies sharing a title (e.g. "The Thing"
// (1982) vs (2011)) would collapse onto the first one created and
// enrichment from the second event would clobber the first record.
// A title-only fallback is preserved for the case where neither side has
// a year yet (e.g. an unenriched record created from a yearless folder).
func findMovie(movies []apiclient.Movie, title string, year int) *apiclient.Movie {
	needle := strings.ToLower(strings.TrimSpace(title))
	var fallback *apiclient.Movie
	for i := range movies {
		if strings.ToLower(strings.TrimSpace(movies[i].Title)) != needle {
			continue
		}
		if year > 0 && movies[i].Year == year {
			return &movies[i]
		}
		// Only consider as a fallback when year info is missing on one
		// side; never associate an incoming yeared event onto a record
		// that has a *different* year.
		if (year == 0 || movies[i].Year == 0) && fallback == nil {
			fallback = &movies[i]
		}
	}
	return fallback
}
