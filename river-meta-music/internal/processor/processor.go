package processor

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"river-meta-music/internal/apiclient"
	"river-meta-music/internal/consumer"
	"river-meta-music/internal/musicbrainz"
)

type Processor struct {
	api *apiclient.Client
	mb  *musicbrainz.Client
}

func New(api *apiclient.Client, mb *musicbrainz.Client) *Processor {
	return &Processor{api: api, mb: mb}
}

func (p *Processor) Handle(event consumer.MediaDiscoveredEvent) error {
	var artist *apiclient.Artist

	if event.MediaID != "" {
		a, err := p.api.GetArtist(event.MediaID)
		if err != nil {
			return fmt.Errorf("get artist %s: %w", event.MediaID, err)
		}
		artist = a
	} else {
		artists, err := p.api.ListArtists(event.LibraryID)
		if err != nil {
			return fmt.Errorf("list artists: %w", err)
		}
		artist = findArtist(artists, event.DirectoryName)
		if artist == nil {
			log.Printf("INFO artist %q not found in library %s, skipping", event.DirectoryName, event.LibraryID)
			return nil
		}
	}

	return p.enrich(artist)
}

func (p *Processor) RefreshByArtistID(artistID string) error {
	artist, err := p.api.GetArtist(artistID)
	if err != nil {
		return fmt.Errorf("get artist %s: %w", artistID, err)
	}
	return p.enrich(artist)
}

func (p *Processor) enrich(artist *apiclient.Artist) error {
	meta, err := p.mb.SearchArtist(artist.Name)
	if err != nil {
		if errors.Is(err, musicbrainz.ErrNotFound) {
			log.Printf("WARN musicbrainz: no results for %q, skipping enrichment", artist.Name)
			p.api.Log("warn", fmt.Sprintf("failed to identify artist %q: no MusicBrainz match", artist.Name))
			return nil
		}
		return fmt.Errorf("musicbrainz search %q: %w", artist.Name, err)
	}

	if _, err := p.api.UpdateArtist(artist.ID, apiclient.ArtistRequest{
		LibraryID: artist.LibraryID,
		Name:      artist.Name,
		Bio:       meta.Bio,
		ImagePath: artist.ImagePath, // preserve: no image source available
	}); err != nil {
		return fmt.Errorf("update artist %s: %w", artist.ID, err)
	}
	log.Printf("INFO enriched artist %q (id=%s)", artist.Name, artist.ID)
	p.api.Log("info", fmt.Sprintf("identified artist %q via MusicBrainz", artist.Name))

	return p.enrichAlbums(artist, meta.MBID)
}

func (p *Processor) enrichAlbums(artist *apiclient.Artist, mbid string) error {
	albums, err := p.api.ListArtistAlbums(artist.ID)
	if err != nil {
		return fmt.Errorf("list albums for artist %s: %w", artist.ID, err)
	}
	if len(albums) == 0 {
		return nil
	}

	mbAlbums, err := p.mb.FetchAlbums(mbid)
	if err != nil {
		if errors.Is(err, musicbrainz.ErrNotFound) {
			log.Printf("WARN musicbrainz: no albums found for artist %q (mbid=%s)", artist.Name, mbid)
			return nil
		}
		return fmt.Errorf("fetch musicbrainz albums for %q: %w", artist.Name, err)
	}

	mbIndex := make(map[string]musicbrainz.AlbumMeta, len(mbAlbums))
	for _, ma := range mbAlbums {
		mbIndex[normalizeTitle(ma.Title)] = ma
	}

	for _, album := range albums {
		ma, ok := mbIndex[normalizeTitle(album.Title)]
		if !ok {
			continue
		}
		if _, err := p.api.UpdateAlbum(album.ID, apiclient.AlbumRequest{
			LibraryID: album.LibraryID,
			ArtistID:  album.ArtistID,
			Title:     album.Title,
			Year:      coalesce(ma.Year, album.Year),
			Genre:     coalesce(ma.Genre, album.Genre),
			CoverPath: coalesce(ma.CoverURL, album.CoverPath),
		}); err != nil {
			log.Printf("WARN update album %q (%s): %v", album.Title, album.ID, err)
			continue
		}
		log.Printf("INFO enriched album %q (id=%s) year=%d genre=%q", album.Title, album.ID, ma.Year, ma.Genre)
	}
	return nil
}

func findArtist(artists []apiclient.Artist, name string) *apiclient.Artist {
	needle := normalizeTitle(name)
	for i := range artists {
		if normalizeTitle(artists[i].Name) == needle {
			return &artists[i]
		}
	}
	return nil
}

func normalizeTitle(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func coalesce[T comparable](a, b T) T {
	var zero T
	if a != zero {
		return a
	}
	return b
}
