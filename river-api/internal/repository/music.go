package repository

import (
	"errors"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

// --- Artists ---

type ArtistRepository interface {
	// FindAll returns a page of artists. See MovieRepository.FindAll for
	// the orderBy contract.
	FindAll(libraryID string, offset, limit int, orderBy string) ([]models.Artist, error)
	FindByID(id string) (*models.Artist, error)
	Create(artist *models.Artist) error
	Save(artist *models.Artist) error
	Delete(id string) error
}

type artistRepository struct{ db *gorm.DB }

func NewArtistRepository(db *gorm.DB) ArtistRepository { return &artistRepository{db} }

func (r *artistRepository) FindAll(libraryID string, offset, limit int, orderBy string) ([]models.Artist, error) {
	var artists []models.Artist
	q := r.db.Model(&models.Artist{})
	if libraryID != "" {
		q = q.Where("library_id = ?", libraryID)
	}
	if orderBy != "" {
		q = q.Order(orderBy)
	}
	return artists, q.Offset(offset).Limit(limit).Find(&artists).Error
}

func (r *artistRepository) FindByID(id string) (*models.Artist, error) {
	var artist models.Artist
	if err := r.db.First(&artist, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &artist, nil
}

func (r *artistRepository) Create(artist *models.Artist) error { return r.db.Create(artist).Error }
func (r *artistRepository) Save(artist *models.Artist) error   { return r.db.Save(artist).Error }
func (r *artistRepository) Delete(id string) error             { return r.db.Delete(&models.Artist{}, "id = ?", id).Error }

// --- Albums ---

type AlbumRepository interface {
	// FindAll returns a page of albums. See MovieRepository.FindAll for
	// the orderBy contract.
	FindAll(libraryID string, offset, limit int, orderBy string) ([]models.Album, error)
	// Count returns the total number of albums matching the libraryID
	// filter applied by FindAll. See MovieRepository.Count.
	Count(libraryID string) (int64, error)
	FindByArtistID(artistID string, offset, limit int) ([]models.Album, error)
	FindByID(id string) (*models.Album, error)
	Create(album *models.Album) error
	Save(album *models.Album) error
	Delete(id string) error
}

type albumRepository struct{ db *gorm.DB }

func NewAlbumRepository(db *gorm.DB) AlbumRepository { return &albumRepository{db} }

func (r *albumRepository) FindAll(libraryID string, offset, limit int, orderBy string) ([]models.Album, error) {
	var albums []models.Album
	q := r.db.Model(&models.Album{})
	if libraryID != "" {
		q = q.Where("library_id = ?", libraryID)
	}
	if orderBy != "" {
		q = q.Order(orderBy)
	}
	return albums, q.Offset(offset).Limit(limit).Find(&albums).Error
}

func (r *albumRepository) Count(libraryID string) (int64, error) {
	var n int64
	q := r.db.Model(&models.Album{})
	if libraryID != "" {
		q = q.Where("library_id = ?", libraryID)
	}
	return n, q.Count(&n).Error
}

func (r *albumRepository) FindByArtistID(artistID string, offset, limit int) ([]models.Album, error) {
	var albums []models.Album
	return albums, r.db.Where("artist_id = ?", artistID).Offset(offset).Limit(limit).Find(&albums).Error
}

func (r *albumRepository) FindByID(id string) (*models.Album, error) {
	var album models.Album
	if err := r.db.First(&album, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &album, nil
}

func (r *albumRepository) Create(album *models.Album) error { return r.db.Create(album).Error }
func (r *albumRepository) Save(album *models.Album) error   { return r.db.Save(album).Error }
func (r *albumRepository) Delete(id string) error           { return r.db.Delete(&models.Album{}, "id = ?", id).Error }

// --- Tracks ---

type TrackRepository interface {
	FindByAlbumID(albumID string, offset, limit int) ([]models.Track, error)
	FindByID(id string) (*models.Track, error)
	Create(track *models.Track) error
	Delete(id string) error
}

type trackRepository struct{ db *gorm.DB }

func NewTrackRepository(db *gorm.DB) TrackRepository { return &trackRepository{db} }

func (r *trackRepository) FindByAlbumID(albumID string, offset, limit int) ([]models.Track, error) {
	var tracks []models.Track
	return tracks, r.db.Where("album_id = ?", albumID).Order("disc_number, number").Offset(offset).Limit(limit).Find(&tracks).Error
}

func (r *trackRepository) FindByID(id string) (*models.Track, error) {
	var track models.Track
	if err := r.db.First(&track, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &track, nil
}

func (r *trackRepository) Create(track *models.Track) error { return r.db.Create(track).Error }
func (r *trackRepository) Delete(id string) error           { return r.db.Delete(&models.Track{}, "id = ?", id).Error }
