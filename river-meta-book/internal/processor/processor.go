package processor

import (
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
	meta, err := p.ol.FetchMetadata(book.Title)
	if err != nil {
		if errors.Is(err, openlib.ErrNotFound) {
			log.Printf("WARN openlib: no results for %q, skipping enrichment", book.Title)
			p.api.Log("warn", fmt.Sprintf("failed to identify audiobook %q: no Open Library match", book.Title))
			return nil
		}
		return fmt.Errorf("openlib fetch %q: %w", book.Title, err)
	}

	if _, err := p.api.UpdateAudiobook(book.ID, apiclient.AudiobookRequest{
		LibraryID:   book.LibraryID,
		Title:       book.Title,
		Author:      meta.Author,
		Narrator:    book.Narrator,  // preserve: not available from Open Library
		Description: meta.Description,
		Year:        coalesce(meta.Year, book.Year),
		Genre:       meta.Genre,
		CoverPath:   meta.CoverURL,
		Duration:    book.Duration, // preserve: computed by audio transcoder
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
