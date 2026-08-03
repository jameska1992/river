package handlers

import (
	"net/http"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type ShowMergeHandler struct {
	svc *services.ShowMergeService
}

func NewShowMergeHandler(svc *services.ShowMergeService) *ShowMergeHandler {
	return &ShowMergeHandler{svc: svc}
}

type mergeRequest struct {
	ShowIDs []string `json:"show_ids" binding:"required"`
}

func (r mergeRequest) valid() bool {
	return len(r.ShowIDs) == 2 && r.ShowIDs[0] != "" && r.ShowIDs[1] != ""
}

// ResolvePath resolves a merged-away directory root to its surviving show.
// Returns 200 with an empty show_id when the path maps to no show.
//
// @Summary      Resolve a show by an absorbed folder path
// @Tags         tvshows
// @Produce      json
// @Param        library_id   query     string  true  "Library id"
// @Param        folder_path  query     string  true  "Directory root to resolve"
// @Success      200          {object}  map[string]string  "show_id is empty when there is no mapping"
// @Router       /admin/tvshows/resolve [get]
func (h *ShowMergeHandler) ResolvePath(c *gin.Context) {
	libraryID := c.Query("library_id")
	folderPath := c.Query("folder_path")
	if libraryID == "" || folderPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "library_id and folder_path are required"})
		return
	}
	id, err := h.svc.ResolveShowByPath(libraryID, folderPath)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"show_id": id})
}

// PreviewMerge reports what merging two shows would do, without changing data.
//
// @Summary      Preview a TV show merge
// @Tags         tvshows
// @Accept       json
// @Produce      json
// @Param        body  body      mergeRequest  true  "Two show ids to merge"
// @Success      200   {object}  services.MergePreview
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Router       /admin/tvshows/merge/preview [post]
func (h *ShowMergeHandler) PreviewMerge(c *gin.Context) {
	var req mergeRequest
	if err := c.ShouldBindJSON(&req); err != nil || !req.valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "exactly two distinct show_ids are required"})
		return
	}
	prev, err := h.svc.PreviewMerge(req.ShowIDs[0], req.ShowIDs[1])
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, prev)
}

// Merge folds the newer of two shows into the older and returns the survivor.
//
// @Summary      Merge two TV shows
// @Tags         tvshows
// @Accept       json
// @Produce      json
// @Param        body  body      mergeRequest  true  "Two show ids to merge"
// @Success      200   {object}  models.TVShow
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Failure      409   {object}  map[string]string  "colliding episodes block the merge"
// @Router       /admin/tvshows/merge [post]
func (h *ShowMergeHandler) Merge(c *gin.Context) {
	var req mergeRequest
	if err := c.ShouldBindJSON(&req); err != nil || !req.valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "exactly two distinct show_ids are required"})
		return
	}
	show, err := h.svc.Merge(req.ShowIDs[0], req.ShowIDs[1])
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, show)
}
