package handlers

import (
	"net/http"
	"strconv"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type MusicHandler struct {
	svc *services.MusicService
}

func NewMusicHandler(svc *services.MusicService) *MusicHandler {
	return &MusicHandler{svc: svc}
}

// --- Artists ---

type artistRequest struct {
	LibraryID string `json:"library_id" binding:"required,uuid"`
	Name      string `json:"name" binding:"required"`
	Bio       string `json:"bio"`
	ImagePath string `json:"image_path"`
}

// ListArtists returns a paginated artist list.
//
// @Summary      List artists
// @Tags         music
// @Produce      json
// @Param        library_id  query  string  false  "Filter by library"
// @Param        page        query  int     false  "Page number"
// @Param        limit       query  int     false  "Page size"
// @Param        sort        query  string  false  "name | added"
// @Param        order       query  string  false  "asc | desc"
// @Success      200  {array}   models.Artist
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /artists [get]
func (h *MusicHandler) ListArtists(c *gin.Context) {
	page, limit := parsePaginationQuery(c)
	artists, err := h.svc.ListArtists(services.ArtistFilter{
		LibraryID: c.Query("library_id"), Page: page, Limit: limit,
		Sort:  c.Query("sort"),
		Order: c.Query("order"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch artists"})
		return
	}
	c.JSON(http.StatusOK, artists)
}

// CreateArtist adds a new artist.
//
// @Summary      Create artist
// @Tags         music
// @Accept       json
// @Produce      json
// @Param        body  body      artistRequest  true  "Artist fields"
// @Success      201   {object}  models.Artist
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /artists [post]
func (h *MusicHandler) CreateArtist(c *gin.Context) {
	var req artistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	libID, err := uuid.Parse(req.LibraryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid library_id"})
		return
	}
	artist, err := h.svc.CreateArtist(services.ArtistInput{LibraryID: libID, Name: req.Name, Bio: req.Bio, ImagePath: req.ImagePath})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create artist"})
		return
	}
	c.JSON(http.StatusCreated, artist)
}

// GetArtist returns a single artist.
//
// @Summary      Get artist
// @Tags         music
// @Produce      json
// @Param        id  path  string  true  "Artist ID"
// @Success      200  {object}  models.Artist
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /artists/{id} [get]
func (h *MusicHandler) GetArtist(c *gin.Context) {
	artist, err := h.svc.GetArtist(c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "artist not found"})
		return
	}
	c.JSON(http.StatusOK, artist)
}

// UpdateArtist replaces artist metadata.
//
// @Summary      Update artist
// @Tags         music
// @Accept       json
// @Produce      json
// @Param        id    path      string         true  "Artist ID"
// @Param        body  body      artistRequest  true  "Artist fields"
// @Success      200   {object}  models.Artist
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Security     BearerAuth
// @Router       /artists/{id} [put]
func (h *MusicHandler) UpdateArtist(c *gin.Context) {
	var req artistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	libID, err := uuid.Parse(req.LibraryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid library_id"})
		return
	}
	artist, err := h.svc.UpdateArtist(c.Param("id"), services.ArtistInput{LibraryID: libID, Name: req.Name, Bio: req.Bio, ImagePath: req.ImagePath})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, artist)
}

// DeleteArtist removes the artist (cascades albums + tracks).
//
// @Summary      Delete artist
// @Tags         music
// @Param        id  path  string  true  "Artist ID"
// @Success      204
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /artists/{id} [delete]
func (h *MusicHandler) DeleteArtist(c *gin.Context) {
	if err := h.svc.DeleteArtist(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete artist"})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// ListArtistAlbums returns the albums for one artist.
//
// @Summary      List artist albums
// @Tags         music
// @Produce      json
// @Param        id     path   string  true   "Artist ID"
// @Param        page   query  int     false  "Page number"
// @Param        limit  query  int     false  "Page size"
// @Success      200    {array}   models.Album
// @Failure      500    {object}  map[string]string
// @Security     BearerAuth
// @Router       /artists/{id}/albums [get]
func (h *MusicHandler) ListArtistAlbums(c *gin.Context) {
	page, limit := parsePaginationQuery(c)
	albums, err := h.svc.ListArtistAlbums(c.Param("id"), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch albums"})
		return
	}
	c.JSON(http.StatusOK, albums)
}

// --- Albums ---

type albumRequest struct {
	LibraryID string `json:"library_id" binding:"required,uuid"`
	ArtistID  string `json:"artist_id"`
	Title     string `json:"title" binding:"required"`
	Year      int    `json:"year"`
	Genre     string `json:"genre"`
	CoverPath string `json:"cover_path"`
}

// ListAlbums returns a paginated album list.
//
// @Summary      List albums
// @Tags         music
// @Produce      json
// @Param        library_id  query  string  false  "Filter by library"
// @Param        page        query  int     false  "Page number"
// @Param        limit       query  int     false  "Page size"
// @Param        sort        query  string  false  "title | year | added"
// @Param        order       query  string  false  "asc | desc"
// @Success      200  {array}   models.Album
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /albums [get]
func (h *MusicHandler) ListAlbums(c *gin.Context) {
	page, limit := parsePaginationQuery(c)
	libraryID := c.Query("library_id")
	albums, err := h.svc.ListAlbums(services.AlbumFilter{
		LibraryID: libraryID, Page: page, Limit: limit,
		Sort:  c.Query("sort"),
		Order: c.Query("order"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch albums"})
		return
	}
	if total, err := h.svc.CountAlbums(libraryID); err == nil {
		c.Header("X-Total-Count", strconv.FormatInt(total, 10))
	}
	c.JSON(http.StatusOK, albums)
}

// CreateAlbum adds a new album.
//
// @Summary      Create album
// @Tags         music
// @Accept       json
// @Produce      json
// @Param        body  body      albumRequest  true  "Album fields"
// @Success      201   {object}  models.Album
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /albums [post]
func (h *MusicHandler) CreateAlbum(c *gin.Context) {
	var req albumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	libID, err := uuid.Parse(req.LibraryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid library_id"})
		return
	}
	input := services.AlbumInput{LibraryID: libID, Title: req.Title, Year: req.Year, Genre: req.Genre, CoverPath: req.CoverPath}
	if req.ArtistID != "" {
		if id, err := uuid.Parse(req.ArtistID); err == nil {
			input.ArtistID = id
		}
	}
	album, err := h.svc.CreateAlbum(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create album"})
		return
	}
	c.JSON(http.StatusCreated, album)
}

// GetAlbum returns a single album.
//
// @Summary      Get album
// @Tags         music
// @Produce      json
// @Param        id  path  string  true  "Album ID"
// @Success      200  {object}  models.Album
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /albums/{id} [get]
func (h *MusicHandler) GetAlbum(c *gin.Context) {
	album, err := h.svc.GetAlbum(c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "album not found"})
		return
	}
	c.JSON(http.StatusOK, album)
}

// UpdateAlbum replaces album metadata.
//
// @Summary      Update album
// @Tags         music
// @Accept       json
// @Produce      json
// @Param        id    path      string        true  "Album ID"
// @Param        body  body      albumRequest  true  "Album fields"
// @Success      200   {object}  models.Album
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Security     BearerAuth
// @Router       /albums/{id} [put]
func (h *MusicHandler) UpdateAlbum(c *gin.Context) {
	var req albumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	libID, err := uuid.Parse(req.LibraryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid library_id"})
		return
	}
	album, err := h.svc.UpdateAlbum(c.Param("id"), services.AlbumInput{LibraryID: libID, Title: req.Title, Year: req.Year, Genre: req.Genre, CoverPath: req.CoverPath})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, album)
}

// DeleteAlbum removes the album (cascades tracks).
//
// @Summary      Delete album
// @Tags         music
// @Param        id  path  string  true  "Album ID"
// @Success      204
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /albums/{id} [delete]
func (h *MusicHandler) DeleteAlbum(c *gin.Context) {
	if err := h.svc.DeleteAlbum(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete album"})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// ListAlbumTracks returns the tracks for one album.
//
// @Summary      List album tracks
// @Tags         music
// @Produce      json
// @Param        id     path   string  true   "Album ID"
// @Param        page   query  int     false  "Page number"
// @Param        limit  query  int     false  "Page size"
// @Success      200    {array}   models.Track
// @Failure      500    {object}  map[string]string
// @Security     BearerAuth
// @Router       /albums/{id}/tracks [get]
func (h *MusicHandler) ListAlbumTracks(c *gin.Context) {
	page, limit := parsePaginationQuery(c)
	tracks, err := h.svc.ListAlbumTracks(c.Param("id"), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tracks"})
		return
	}
	c.JSON(http.StatusOK, tracks)
}

// --- Tracks ---

type trackRequest struct {
	LibraryID  string `json:"library_id" binding:"required,uuid"`
	AlbumID    string `json:"album_id" binding:"required,uuid"`
	ArtistID   string `json:"artist_id"`
	Title      string `json:"title" binding:"required"`
	Number     int    `json:"number"`
	DiscNumber int    `json:"disc_number"`
	Duration   int    `json:"duration"`
	FilePath   string `json:"file_path" binding:"required"`
}

// CreateTrack adds a new track.
//
// @Summary      Create track
// @Tags         music
// @Accept       json
// @Produce      json
// @Param        body  body      trackRequest  true  "Track fields"
// @Success      201   {object}  models.Track
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /tracks [post]
func (h *MusicHandler) CreateTrack(c *gin.Context) {
	var req trackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	libID, err := uuid.Parse(req.LibraryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid library_id"})
		return
	}
	albumID, err := uuid.Parse(req.AlbumID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid album_id"})
		return
	}
	input := services.TrackInput{
		LibraryID: libID, AlbumID: albumID, Title: req.Title,
		Number: req.Number, DiscNumber: req.DiscNumber, Duration: req.Duration, FilePath: req.FilePath,
	}
	if req.ArtistID != "" {
		if id, err := uuid.Parse(req.ArtistID); err == nil {
			input.ArtistID = id
		}
	}
	track, err := h.svc.CreateTrack(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create track"})
		return
	}
	c.JSON(http.StatusCreated, track)
}

// GetTrack returns a single track.
//
// @Summary      Get track
// @Tags         music
// @Produce      json
// @Param        id  path  string  true  "Track ID"
// @Success      200  {object}  models.Track
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tracks/{id} [get]
func (h *MusicHandler) GetTrack(c *gin.Context) {
	track, err := h.svc.GetTrack(c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "track not found"})
		return
	}
	c.JSON(http.StatusOK, track)
}

// DeleteTrack removes the track.
//
// @Summary      Delete track
// @Tags         music
// @Param        id  path  string  true  "Track ID"
// @Success      204
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tracks/{id} [delete]
func (h *MusicHandler) DeleteTrack(c *gin.Context) {
	if err := h.svc.DeleteTrack(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete track"})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// StreamTrack serves the track file with Range support.
//
// @Summary      Stream track
// @Tags         music
// @Produce      audio/mp4
// @Param        id     path   string  true   "Track ID"
// @Param        token  query  string  false  "Stream JWT"
// @Success      200
// @Success      206
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tracks/{id}/stream [get]
func (h *MusicHandler) StreamTrack(c *gin.Context) {
	track, err := h.svc.GetTrack(c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "track not found"})
		return
	}
	serveMediaFile(c, track.FilePath)
}
