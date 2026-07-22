package services

import (
	"river-api/internal/models"
	"river-api/internal/repository"
)

type SearchService struct {
	repo repository.SearchRepository
}

func NewSearchService(repo repository.SearchRepository) *SearchService {
	return &SearchService{repo: repo}
}

type SearchResultItem struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Year       int    `json:"year"`
	PosterPath string `json:"poster_path"`
	MediaType  string `json:"media_type"` // "movie" or "tvshow"
}

type LibrarySearchResult struct {
	LibraryID   string             `json:"library_id"`
	LibraryName string             `json:"library_name"`
	LibraryType string             `json:"library_type"`
	Items       []SearchResultItem `json:"items"`
}

type PersonSearchResult struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ProfilePath string `json:"profile_path"`
}

type SearchResult struct {
	Libraries []LibrarySearchResult `json:"libraries"`
	People    []PersonSearchResult  `json:"people"`
}

func (s *SearchService) Search(query, genre string) (*SearchResult, error) {
	movies, err := s.repo.SearchMovies(query, genre, 50)
	if err != nil {
		return nil, err
	}
	shows, err := s.repo.SearchTVShows(query, genre, 50)
	if err != nil {
		return nil, err
	}
	audiobooks, err := s.repo.SearchAudiobooks(query, genre, 50)
	if err != nil {
		return nil, err
	}
	// People have no genres; skip people lookup for genre-only searches.
	var people []models.Person
	if query != "" {
		people, err = s.repo.SearchPeople(query, 15)
		if err != nil {
			return nil, err
		}
	}

	// Group items by library, preserving insertion order.
	libMap := make(map[string]*LibrarySearchResult)
	libOrder := make([]string, 0)

	add := func(libID, libName, libType string, item SearchResultItem) {
		if _, ok := libMap[libID]; !ok {
			libMap[libID] = &LibrarySearchResult{
				LibraryID:   libID,
				LibraryName: libName,
				LibraryType: libType,
				Items:       []SearchResultItem{},
			}
			libOrder = append(libOrder, libID)
		}
		libMap[libID].Items = append(libMap[libID].Items, item)
	}

	for _, m := range movies {
		add(m.LibraryID.String(), m.Library.Name, string(m.Library.Type), SearchResultItem{
			ID: m.ID.String(), Title: m.Title, Year: m.Year, PosterPath: m.PosterPath, MediaType: "movie",
		})
	}
	for _, sh := range shows {
		add(sh.LibraryID.String(), sh.Library.Name, string(sh.Library.Type), SearchResultItem{
			ID: sh.ID.String(), Title: sh.Title, Year: sh.Year, PosterPath: sh.PosterPath, MediaType: "tvshow",
		})
	}
	for _, b := range audiobooks {
		add(b.LibraryID.String(), b.Library.Name, string(b.Library.Type), SearchResultItem{
			ID: b.ID.String(), Title: b.Title, Year: b.Year, PosterPath: b.CoverPath, MediaType: "audiobook",
		})
	}

	libs := make([]LibrarySearchResult, 0, len(libOrder))
	for _, id := range libOrder {
		libs = append(libs, *libMap[id])
	}

	ppl := make([]PersonSearchResult, len(people))
	for i, p := range people {
		ppl[i] = PersonSearchResult{ID: p.ID.String(), Name: p.Name, ProfilePath: p.ProfilePath}
	}

	return &SearchResult{Libraries: libs, People: ppl}, nil
}
