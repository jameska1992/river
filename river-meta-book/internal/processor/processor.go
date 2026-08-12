package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"river-meta-book/internal/apiclient"
	"river-meta-book/internal/consumer"
	"river-meta-book/internal/openlib"
)

var dirPattern = regexp.MustCompile(`^(.+?)\s*\((\d{4})\)$`)

type Processor struct {
	api *apiclient.Client
	ol  *openlib.Client
}

func New(api *apiclient.Client, ol *openlib.Client) *Processor {
	return &Processor{api: api, ol: ol}
}

func (p *Processor) Handle(event consumer.MediaDiscoveredEvent) error {
	var book *apiclient.Audiobook
	if event.MediaID != "" {
		b, err := p.api.GetAudiobook(event.MediaID)
		if err != nil {
			return fmt.Errorf("get audiobook %s: %w", event.MediaID, err)
		}
		book = b
	} else {
		title, _ := parseDirectoryName(event.DirectoryName)
		books, err := p.api.ListAudiobooks(event.LibraryID)
		if err != nil {
			return fmt.Errorf("list audiobooks: %w", err)
		}
		book = findAudiobook(books, title)
		if book == nil {
			log.Printf("INFO audiobook %q not found in library %s, skipping", title, event.LibraryID)
			return nil
		}
	}
	return p.enrich(book)
}

func (p *Processor) RefreshByID(id string) error {
	book, err := p.api.GetAudiobook(id)
	if err != nil {
		return fmt.Errorf("get audiobook %s: %w", id, err)
	}
	return p.enrich(book)
}

func (p *Processor) enrich(book *apiclient.Audiobook) error {
	// Sticky matching: once a book is pinned to an Open Library work, resolve by
	// that key so a rescan can't drift to a different title-search result. Only
	// fall back to a fresh title search when no key is stored yet.
	var (
		meta *openlib.Metadata
		err  error
	)
	if book.OpenLibraryKey != "" {
		meta, err = p.ol.FetchByWorkKey(book.OpenLibraryKey)
	} else {
		meta, err = p.ol.FetchMetadata(book.Title)
	}
	if err != nil {
		if errors.Is(err, openlib.ErrNotFound) {
			log.Printf("WARN openlib: no results for %q, skipping enrichment", book.Title)
			p.api.Log("warn", fmt.Sprintf("failed to identify audiobook %q: no Open Library match", book.Title))
			return nil
		}
		return fmt.Errorf("openlib fetch %q: %w", book.Title, err)
	}

	// ISBNs: JSON-encode when present; leave empty otherwise so river-api's
	// sticky Update preserves any previously-stored value rather than clearing it.
	isbns := ""
	if len(meta.ISBNs) > 0 {
		if b, err := json.Marshal(meta.ISBNs); err == nil {
			isbns = string(b)
		}
	}

	if _, err := p.api.UpdateAudiobook(book.ID, apiclient.AudiobookRequest{
		LibraryID: book.LibraryID,
		Title:     book.Title,
		// Coalesce the enriched fields against what's already on the record:
		// only overwrite when Open Library returned a non-empty value, so a
		// sparse result (or a later fetch-by-key) can't blank an admin's edit.
		Author:      coalesceStr(meta.Author, book.Author),
		Narrator:    book.Narrator, // preserve: not available from Open Library
		Description: coalesceStr(meta.Description, book.Description),
		Year:        coalesce(meta.Year, book.Year),
		Genre:       coalesceStr(meta.Genre, book.Genre),
		CoverPath:   coalesceStr(meta.CoverURL, book.CoverPath),
		Duration:    book.Duration, // preserve: computed by audio transcoder
		// Sticky identifiers. Empty values are omitted / preserved server-side.
		OpenLibraryKey: meta.WorkKey,
		ISBNs:          isbns,
	}); err != nil {
		return fmt.Errorf("update audiobook %s: %w", book.ID, err)
	}

	log.Printf("INFO enriched audiobook %q (id=%s) author=%q", book.Title, book.ID, meta.Author)
	authorTag := ""
	if meta.Author != "" {
		authorTag = fmt.Sprintf(" by %s", meta.Author)
	}
	p.api.Log("info", fmt.Sprintf("identified audiobook %q%s via Open Library", book.Title, authorTag))
	return nil
}

func parseDirectoryName(name string) (string, int) {
	m := dirPattern.FindStringSubmatch(name)
	if m == nil {
		return name, 0
	}
	year, _ := strconv.Atoi(m[2])
	return strings.TrimSpace(m[1]), year
}

func findAudiobook(books []apiclient.Audiobook, title string) *apiclient.Audiobook {
	needle := strings.ToLower(strings.TrimSpace(title))
	for i := range books {
		if strings.ToLower(strings.TrimSpace(books[i].Title)) == needle {
			return &books[i]
		}
	}
	return nil
}

func coalesce(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

func coalesceStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
