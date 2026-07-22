package handlers

import (
	"encoding/json"
	"net/http"

	"river-api/internal/middleware"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// isValidProgressMediaType gates which media types may report watch
// progress. Movies and TV episodes are obvious; "chapter" is the per-
// audiobook-chapter equivalent — the audiobook player reports against
// the chapter currently playing, which gives the same fidelity for the
// continue-watching and active-sessions surfaces.
func isValidProgressMediaType(t string) bool {
	switch t {
	case "movie", "episode", "chapter":
		return true
	}
	return false
}

type ProgressWSHandler struct {
	svc *services.ProgressService
}

func NewProgressWSHandler(svc *services.ProgressService) *ProgressWSHandler {
	return &ProgressWSHandler{svc: svc}
}

// ServeWS upgrades the connection to a WebSocket that accepts streaming
// progress updates. Each frame is a JSON {media_type, media_id,
// position, duration} write that's batched into the DB by the service.
//
// @Summary      Watch progress WebSocket
// @Tags         progress
// @Param        token  query  string  false  "Access JWT (also accepted via Authorization header)"
// @Success      101    "Switching Protocols"
// @Failure      401    {object}  map[string]string
// @Security     BearerAuth
// @Router       /progress/ws [get]
func (h *ProgressWSHandler) ServeWS(c *gin.Context) {
	claims := middleware.GetClaims(c)
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if mt != websocket.TextMessage {
			continue
		}
		var req progressRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			continue
		}
		if req.MediaID == "" || !isValidProgressMediaType(req.MediaType) {
			continue
		}
		h.svc.Report(services.ProgressInput{
			UserID:    claims.UserID,
			MediaType: req.MediaType,
			MediaID:   req.MediaID,
			Position:  req.Position,
			Duration:  req.Duration,
		})
	}
}
