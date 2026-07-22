package handlers

import (
	"net/http"

	"river-api/internal/middleware"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

// CollectionHandler — collections are global. Reads and writes require
// authentication but not a specific user; the JWT only matters for
// Create, where it stamps "created by" on the new record. Any other
// authenticated user can edit or delete it afterwards.

type CollectionHandler struct {
	svc *services.CollectionService
}

func NewCollectionHandler(svc *services.CollectionService) *CollectionHandler {
	return &CollectionHandler{svc: svc}
}

type collectionRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type collectionItemRequest struct {
	MediaType string `json:"media_type" binding:"required,oneof=movie tvshow audiobook"`
	MediaID   string `json:"media_id" binding:"required"`
}

// List returns all collections.
//
// @Summary      List collections
// @Tags         collections
// @Produce      json
// @Success      200  {array}   object
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /collections [get]
func (h *CollectionHandler) List(c *gin.Context) {
	summaries, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch collections"})
		return
	}
	c.JSON(http.StatusOK, summaries)
}

// Create adds a new collection, owned by the calling user.
//
// @Summary      Create collection
// @Tags         collections
// @Accept       json
// @Produce      json
// @Param        body  body      collectionRequest  true  "Collection fields"
// @Success      201   {object}  models.Collection
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /collections [post]
func (h *CollectionHandler) Create(c *gin.Context) {
	claims := middleware.GetClaims(c)
	var req collectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	col, err := h.svc.Create(claims.UserID, req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create collection"})
		return
	}
	c.JSON(http.StatusCreated, col)
}

// Get returns a collection with its items.
//
// @Summary      Get collection
// @Tags         collections
// @Produce      json
// @Param        id  path  string  true  "Collection ID"
// @Success      200  {object}  object
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /collections/{id} [get]
func (h *CollectionHandler) Get(c *gin.Context) {
	detail, err := h.svc.GetWithItems(c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "collection not found"})
		return
	}
	c.JSON(http.StatusOK, detail)
}

// Update changes a collection's name/description.
//
// @Summary      Update collection
// @Tags         collections
// @Accept       json
// @Produce      json
// @Param        id    path      string             true  "Collection ID"
// @Param        body  body      collectionRequest  true  "Collection fields"
// @Success      200   {object}  models.Collection
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Security     BearerAuth
// @Router       /collections/{id} [put]
func (h *CollectionHandler) Update(c *gin.Context) {
	var req collectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	col, err := h.svc.Update(c.Param("id"), req.Name, req.Description)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, col)
}

// Delete removes a collection.
//
// @Summary      Delete collection
// @Tags         collections
// @Param        id  path  string  true  "Collection ID"
// @Success      204
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /collections/{id} [delete]
func (h *CollectionHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Param("id")); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "failed to delete collection"})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// AddItem adds a movie/tvshow/audiobook to a collection.
//
// @Summary      Add collection item
// @Tags         collections
// @Accept       json
// @Produce      json
// @Param        id    path      string                 true  "Collection ID"
// @Param        body  body      collectionItemRequest  true  "Item reference"
// @Success      201   {object}  models.CollectionItem
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Security     BearerAuth
// @Router       /collections/{id}/items [post]
func (h *CollectionHandler) AddItem(c *gin.Context) {
	var req collectionItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.AddItem(c.Param("id"), req.MediaType, req.MediaID)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, item)
}

// RemoveItem removes an item from a collection.
//
// @Summary      Remove collection item
// @Tags         collections
// @Param        id      path  string  true  "Collection ID"
// @Param        itemId  path  string  true  "Item ID"
// @Success      204
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /collections/{id}/items/{itemId} [delete]
func (h *CollectionHandler) RemoveItem(c *gin.Context) {
	if err := h.svc.RemoveItem(c.Param("id"), c.Param("itemId")); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "failed to remove item"})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
