package services

import (
	"river-api/internal/models"
	"river-api/internal/repository"

	"github.com/google/uuid"
)

type MusicService struct {
	artists repository.ArtistRepository
	albums  repository.AlbumRepository
	tracks  repository.TrackRepository
}

func NewMusicService(
	artists repository.ArtistRepository,
	albums repository.AlbumRepository,
	tracks repository.TrackRepository,
) *MusicService {
	return &MusicService{artists: artists, albums: albums, tracks: tracks}
}

// --- Artists ---

type ArtistFilter struct {
	LibraryID string
	Page      int
	Limit     int
	Sort      string // name | added (default: name)
	Order     string // asc | desc (default: asc)
}

var artistSortColumns = map[string]string{
	"name":  titleSortExpr("name"),
	"added": "created_at",
}

type ArtistInput struct {
	LibraryID uuid.UUID
	Name      string
	Bio       string
	ImagePath string
}

func (s *MusicService) ListArtists(f ArtistFilter) ([]models.Artist, error) {
	offset, limit := paginationOffsetLimit(f.Page, f.Limit)
	return s.artists.FindAll(f.LibraryID, offset, limit, sortClause(f.Sort, f.Order, artistSortColumns, titleSortExpr("name")))
}

func (s *MusicService) CreateArtist(input ArtistInput) (*models.Artist, error) {
	artist := models.Artist{LibraryID: input.LibraryID, Name: input.Name, Bio: input.Bio, ImagePath: input.ImagePath}
	return &artist, s.artists.Create(&artist)
}

func (s *MusicService) GetArtist(id string) (*models.Artist, error) {
	return s.artists.FindByID(id)
}

func (s *MusicService) UpdateArtist(id string, input ArtistInput) (*models.Artist, error) {
	artist, err := s.artists.FindByID(id)
	if err != nil {
		return nil, err
	}
	artist.Name = input.Name
	artist.Bio = input.Bio
	artist.ImagePath = input.ImagePath
	return artist, s.artists.Save(artist)
}

func (s *MusicService) DeleteArtist(id string) error {
	return s.artists.Delete(id)
}

func (s *MusicService) ListArtistAlbums(artistID string, page, limit int) ([]models.Album, error) {
	offset, lim := paginationOffsetLimit(page, limit)
	return s.albums.FindByArtistID(artistID, offset, lim)
}

// --- Albums ---

type AlbumFilter struct {
	LibraryID string
	Page      int
	Limit     int
	Sort      string // title | year | added (default: title)
	Order     string // asc | desc (default: asc)
}

var albumSortColumns = map[string]string{
	"title": titleSortExpr("title"),
	"year":  "year",
	"added": "created_at",
}

type AlbumInput struct {
	LibraryID uuid.UUID
	ArtistID  uuid.UUID
	Title     string
	Year      int
	Genre     string
	CoverPath string
}

func (s *MusicService) ListAlbums(f AlbumFilter) ([]models.Album, error) {
	offset, limit := paginationOffsetLimit(f.Page, f.Limit)
	return s.albums.FindAll(f.LibraryID, offset, limit, sortClause(f.Sort, f.Order, albumSortColumns, titleSortExpr("title")))
}

func (s *MusicService) CountAlbums(libraryID string) (int64, error) {
	return s.albums.Count(libraryID)
}

func (s *MusicService) CreateAlbum(input AlbumInput) (*models.Album, error) {
	album := models.Album{
		LibraryID: input.LibraryID, ArtistID: input.ArtistID,
		Title: input.Title, Year: input.Year, Genre: input.Genre, CoverPath: input.CoverPath,
	}
	return &album, s.albums.Create(&album)
}

func (s *MusicService) GetAlbum(id string) (*models.Album, error) {
	return s.albums.FindByID(id)
}

func (s *MusicService) UpdateAlbum(id string, input AlbumInput) (*models.Album, error) {
	album, err := s.albums.FindByID(id)
	if err != nil {
		return nil, err
	}
	album.Title = input.Title
	album.Year = input.Year
	album.Genre = input.Genre
	album.CoverPath = input.CoverPath
	return album, s.albums.Save(album)
}

func (s *MusicService) DeleteAlbum(id string) error {
	return s.albums.Delete(id)
}

func (s *MusicService) ListAlbumTracks(albumID string, page, limit int) ([]models.Track, error) {
	offset, lim := paginationOffsetLimit(page, limit)
	return s.tracks.FindByAlbumID(albumID, offset, lim)
}

// --- Tracks ---

type TrackInput struct {
	LibraryID  uuid.UUID
	AlbumID    uuid.UUID
	ArtistID   uuid.UUID
	Title      string
	Number     int
	DiscNumber int
	Duration   int
	FilePath   string
}

func (s *MusicService) CreateTrack(input TrackInput) (*models.Track, error) {
	track := models.Track{
		LibraryID: input.LibraryID, AlbumID: input.AlbumID, ArtistID: input.ArtistID,
		Title: input.Title, Number: input.Number, DiscNumber: input.DiscNumber,
		Duration: input.Duration, FilePath: input.FilePath,
	}
	return &track, s.tracks.Create(&track)
}

func (s *MusicService) GetTrack(id string) (*models.Track, error) {
	return s.tracks.FindByID(id)
}

func (s *MusicService) DeleteTrack(id string) error {
	return s.tracks.Delete(id)
}
