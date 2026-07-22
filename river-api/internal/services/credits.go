package services

import (
	"river-api/internal/models"
	"river-api/internal/repository"

	"github.com/google/uuid"
)

type CreditsService struct {
	repo repository.CreditsRepository
}

func NewCreditsService(repo repository.CreditsRepository) *CreditsService {
	return &CreditsService{repo: repo}
}

type CastInput struct {
	TmdbID      int // 0 means no TMDB ID — always creates a new person
	Name        string
	ProfilePath string
	Biography   string
	Character   string
	Order       int
}

type CrewInput struct {
	TmdbID      int
	Name        string
	ProfilePath string
	Biography   string
	Job         string
	Department  string
}

type CastResult struct {
	PersonID    uuid.UUID `json:"person_id"`
	TmdbID      *int      `json:"tmdb_id"`
	Name        string    `json:"name"`
	ProfilePath string    `json:"profile_path"`
	Character   string    `json:"character"`
	Order       int       `json:"order"`
}

type CrewResult struct {
	PersonID    uuid.UUID `json:"person_id"`
	TmdbID      *int      `json:"tmdb_id"`
	Name        string    `json:"name"`
	ProfilePath string    `json:"profile_path"`
	Job         string    `json:"job"`
	Department  string    `json:"department"`
}

type CreditsResult struct {
	Cast []CastResult `json:"cast"`
	Crew []CrewResult `json:"crew"`
}

type PersonMovieCastItem struct {
	MovieID    string `json:"movie_id"`
	Title      string `json:"title"`
	Year       int    `json:"year"`
	PosterPath string `json:"poster_path"`
	Character  string `json:"character"`
}

type PersonMovieCrewItem struct {
	MovieID    string `json:"movie_id"`
	Title      string `json:"title"`
	Year       int    `json:"year"`
	PosterPath string `json:"poster_path"`
	Job        string `json:"job"`
	Department string `json:"department"`
}

type PersonTVShowCastItem struct {
	TVShowID   string `json:"tv_show_id"`
	Title      string `json:"title"`
	Year       int    `json:"year"`
	PosterPath string `json:"poster_path"`
	Character  string `json:"character"`
}

type PersonTVShowCrewItem struct {
	TVShowID   string `json:"tv_show_id"`
	Title      string `json:"title"`
	Year       int    `json:"year"`
	PosterPath string `json:"poster_path"`
	Job        string `json:"job"`
	Department string `json:"department"`
}

type PersonResult struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	ProfilePath string                 `json:"profile_path"`
	Biography   string                 `json:"biography"`
	TmdbID      *int                   `json:"tmdb_id"`
	MovieCast   []PersonMovieCastItem  `json:"movie_cast"`
	MovieCrew   []PersonMovieCrewItem  `json:"movie_crew"`
	TVShowCast  []PersonTVShowCastItem `json:"tv_show_cast"`
	TVShowCrew  []PersonTVShowCrewItem `json:"tv_show_crew"`
}

func (s *CreditsService) GetPerson(personID string) (*PersonResult, error) {
	id, err := uuid.Parse(personID)
	if err != nil {
		return nil, ErrNotFound
	}
	p, err := s.repo.FindPersonByID(id)
	if err != nil {
		return nil, err
	}
	mc, mw, sc, sw, err := s.repo.GetPersonFilmography(id)
	if err != nil {
		return nil, err
	}

	res := &PersonResult{
		ID:          p.ID.String(),
		Name:        p.Name,
		ProfilePath: p.ProfilePath,
		Biography:   p.Biography,
		TmdbID:      p.TmdbID,
		MovieCast:   make([]PersonMovieCastItem, len(mc)),
		MovieCrew:   make([]PersonMovieCrewItem, len(mw)),
		TVShowCast:  make([]PersonTVShowCastItem, len(sc)),
		TVShowCrew:  make([]PersonTVShowCrewItem, len(sw)),
	}
	for i, r := range mc {
		res.MovieCast[i] = PersonMovieCastItem{MovieID: r.MovieID, Title: r.Title, Year: r.Year, PosterPath: r.PosterPath, Character: r.Character}
	}
	for i, r := range mw {
		res.MovieCrew[i] = PersonMovieCrewItem{MovieID: r.MovieID, Title: r.Title, Year: r.Year, PosterPath: r.PosterPath, Job: r.Job, Department: r.Department}
	}
	for i, r := range sc {
		res.TVShowCast[i] = PersonTVShowCastItem{TVShowID: r.TVShowID, Title: r.Title, Year: r.Year, PosterPath: r.PosterPath, Character: r.Character}
	}
	for i, r := range sw {
		res.TVShowCrew[i] = PersonTVShowCrewItem{TVShowID: r.TVShowID, Title: r.Title, Year: r.Year, PosterPath: r.PosterPath, Job: r.Job, Department: r.Department}
	}
	return res, nil
}

func (s *CreditsService) SetMovieCredits(movieID string, cast []CastInput, crew []CrewInput) error {
	id, err := uuid.Parse(movieID)
	if err != nil {
		return ErrNotFound
	}
	mc, mcrw, err := s.resolveCredits(cast, crew)
	if err != nil {
		return err
	}
	castModels := make([]models.MovieCast, len(mc))
	for i, c := range mc {
		castModels[i] = models.MovieCast{MovieID: id, PersonID: c.personID, Character: c.character, CastOrder: c.order}
	}
	crewModels := make([]models.MovieCrew, len(mcrw))
	for i, c := range mcrw {
		crewModels[i] = models.MovieCrew{MovieID: id, PersonID: c.personID, Job: c.job, Department: c.department}
	}
	return s.repo.SetMovieCredits(id, castModels, crewModels)
}

func (s *CreditsService) GetMovieCredits(movieID string) (*CreditsResult, error) {
	id, err := uuid.Parse(movieID)
	if err != nil {
		return nil, ErrNotFound
	}
	cast, crew, err := s.repo.GetMovieCredits(id)
	if err != nil {
		return nil, err
	}
	res := &CreditsResult{
		Cast: make([]CastResult, len(cast)),
		Crew: make([]CrewResult, len(crew)),
	}
	for i, c := range cast {
		res.Cast[i] = CastResult{
			PersonID: c.PersonID, TmdbID: c.Person.TmdbID,
			Name: c.Person.Name, ProfilePath: c.Person.ProfilePath,
			Character: c.Character, Order: c.CastOrder,
		}
	}
	for i, c := range crew {
		res.Crew[i] = CrewResult{
			PersonID: c.PersonID, TmdbID: c.Person.TmdbID,
			Name: c.Person.Name, ProfilePath: c.Person.ProfilePath,
			Job: c.Job, Department: c.Department,
		}
	}
	return res, nil
}

func (s *CreditsService) SetTVShowCredits(showID string, cast []CastInput, crew []CrewInput) error {
	id, err := uuid.Parse(showID)
	if err != nil {
		return ErrNotFound
	}
	mc, mcrw, err := s.resolveCredits(cast, crew)
	if err != nil {
		return err
	}
	castModels := make([]models.TVShowCast, len(mc))
	for i, c := range mc {
		castModels[i] = models.TVShowCast{TVShowID: id, PersonID: c.personID, Character: c.character, CastOrder: c.order}
	}
	crewModels := make([]models.TVShowCrew, len(mcrw))
	for i, c := range mcrw {
		crewModels[i] = models.TVShowCrew{TVShowID: id, PersonID: c.personID, Job: c.job, Department: c.department}
	}
	return s.repo.SetTVShowCredits(id, castModels, crewModels)
}

func (s *CreditsService) GetTVShowCredits(showID string) (*CreditsResult, error) {
	id, err := uuid.Parse(showID)
	if err != nil {
		return nil, ErrNotFound
	}
	cast, crew, err := s.repo.GetTVShowCredits(id)
	if err != nil {
		return nil, err
	}
	res := &CreditsResult{
		Cast: make([]CastResult, len(cast)),
		Crew: make([]CrewResult, len(crew)),
	}
	for i, c := range cast {
		res.Cast[i] = CastResult{
			PersonID: c.PersonID, TmdbID: c.Person.TmdbID,
			Name: c.Person.Name, ProfilePath: c.Person.ProfilePath,
			Character: c.Character, Order: c.CastOrder,
		}
	}
	for i, c := range crew {
		res.Crew[i] = CrewResult{
			PersonID: c.PersonID, TmdbID: c.Person.TmdbID,
			Name: c.Person.Name, ProfilePath: c.Person.ProfilePath,
			Job: c.Job, Department: c.Department,
		}
	}
	return res, nil
}

type resolvedCast struct {
	personID  uuid.UUID
	character string
	order     int
}

type resolvedCrew struct {
	personID   uuid.UUID
	job        string
	department string
}

func (s *CreditsService) resolveCredits(cast []CastInput, crew []CrewInput) ([]resolvedCast, []resolvedCrew, error) {
	rc := make([]resolvedCast, 0, len(cast))
	for _, c := range cast {
		p, err := s.findOrCreatePerson(c.TmdbID, c.Name, c.ProfilePath, c.Biography)
		if err != nil {
			return nil, nil, err
		}
		rc = append(rc, resolvedCast{personID: p.ID, character: c.Character, order: c.Order})
	}
	rw := make([]resolvedCrew, 0, len(crew))
	for _, c := range crew {
		p, err := s.findOrCreatePerson(c.TmdbID, c.Name, c.ProfilePath, c.Biography)
		if err != nil {
			return nil, nil, err
		}
		rw = append(rw, resolvedCrew{personID: p.ID, job: c.Job, department: c.Department})
	}
	return rc, rw, nil
}

func (s *CreditsService) findOrCreatePerson(tmdbID int, name, profilePath, biography string) (*models.Person, error) {
	if tmdbID > 0 {
		return s.repo.FindOrCreatePersonByTmdbID(tmdbID, name, profilePath, biography)
	}
	return s.repo.CreatePerson(name, profilePath)
}
