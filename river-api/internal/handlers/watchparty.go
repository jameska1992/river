package handlers

import (
	"encoding/json"
	"net/http"
	"sync"

	"river-api/internal/middleware"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ── Hub ───────────────────────────────────────────────────────────────────────

const wsSendBuf = 16

type wpMember struct {
	userID, username string
	send             chan []byte
}

type wpRoom struct {
	mu       sync.Mutex
	id       string
	hostID   string
	position float64
	playing  bool
	members  map[string]*wpMember
}

type WatchPartyHub struct {
	mu    sync.RWMutex
	rooms map[string]*wpRoom
}

func NewWatchPartyHub() *WatchPartyHub {
	return &WatchPartyHub{rooms: make(map[string]*wpRoom)}
}

func (h *WatchPartyHub) getOrCreate(id, hostID string) *wpRoom {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.rooms[id]; ok {
		return r
	}
	r := &wpRoom{id: id, hostID: hostID, members: make(map[string]*wpMember)}
	h.rooms[id] = r
	return r
}

func (h *WatchPartyHub) delete(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms, id)
}

func (r *wpRoom) addMember(m *wpMember) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.members[m.userID] = m
}

func (r *wpRoom) removeMember(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.members, userID)
}

func (r *wpRoom) broadcast(msg []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.members {
		select {
		case m.send <- msg:
		default:
		}
	}
}

func (r *wpRoom) snapshot() (pos float64, playing bool, list []map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pos, playing = r.position, r.playing
	for _, m := range r.members {
		list = append(list, map[string]string{"user_id": m.userID, "username": m.username})
	}
	return
}

func (r *wpRoom) setState(pos float64, playing bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.position = pos
	r.playing = playing
}

func (r *wpRoom) setPosition(pos float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.position = pos
}

// ── Handler ───────────────────────────────────────────────────────────────────

type WatchPartyHandler struct {
	svc *services.WatchPartyService
	hub *WatchPartyHub
}

func NewWatchPartyHandler(svc *services.WatchPartyService, hub *WatchPartyHub) *WatchPartyHandler {
	return &WatchPartyHandler{svc: svc, hub: hub}
}

type watchPartyCreateReq struct {
	MediaType string `json:"media_type" binding:"required"`
	MediaID   string `json:"media_id" binding:"required"`
	ShowID    string `json:"show_id"`
	SeasonID  string `json:"season_id"`
}

// Create starts a new watch party. The caller becomes the host.
//
// @Summary      Create watch party
// @Tags         watchparty
// @Accept       json
// @Produce      json
// @Param        body  body      watchPartyCreateReq  true  "Party details"
// @Success      201   {object}  models.WatchParty
// @Failure      400   {object}  map[string]string
// @Security     BearerAuth
// @Router       /watchparty [post]
func (h *WatchPartyHandler) Create(c *gin.Context) {
	claims := middleware.GetClaims(c)
	var req watchPartyCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	party, err := h.svc.Create(services.WatchPartyInput{
		HostID:    claims.UserID,
		MediaType: req.MediaType,
		MediaID:   req.MediaID,
		ShowID:    req.ShowID,
		SeasonID:  req.SeasonID,
	})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, party)
}

// Get returns a watch party by ID.
//
// @Summary      Get watch party
// @Tags         watchparty
// @Produce      json
// @Param        id  path  string  true  "Watch party ID"
// @Success      200  {object}  models.WatchParty
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /watchparty/{id} [get]
func (h *WatchPartyHandler) Get(c *gin.Context) {
	party, err := h.svc.GetByID(c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, party)
}

// Delete closes a watch party. Only the host can delete; members are
// notified via the WebSocket before the room is removed.
//
// @Summary      Delete watch party
// @Tags         watchparty
// @Param        id  path  string  true  "Watch party ID"
// @Success      204
// @Failure      403  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /watchparty/{id} [delete]
func (h *WatchPartyHandler) Delete(c *gin.Context) {
	claims := middleware.GetClaims(c)
	id := c.Param("id")

	// Notify connected members before removing the DB record
	h.hub.mu.RLock()
	room := h.hub.rooms[id]
	h.hub.mu.RUnlock()
	if room != nil {
		msg, _ := json.Marshal(map[string]string{"type": "closed"})
		room.broadcast(msg)
		h.hub.delete(id)
	}

	if err := h.svc.Delete(claims.UserID, id); err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ── WebSocket ─────────────────────────────────────────────────────────────────

type wpClientMsg struct {
	Type     string  `json:"type"`
	Position float64 `json:"position"`
}

// ServeWS upgrades the connection to a watch-party WebSocket. Messages
// are {type, position} frames that get broadcast to other members.
//
// @Summary      Watch party WebSocket
// @Tags         watchparty
// @Param        id     path   string  true   "Watch party ID"
// @Param        token  query  string  false  "Access JWT (also accepted via Authorization header)"
// @Success      101
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /watchparty/{id}/ws [get]
func (h *WatchPartyHandler) ServeWS(c *gin.Context) {
	claims := middleware.GetClaims(c)
	roomID := c.Param("id")

	party, err := h.svc.GetByID(roomID)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	room := h.hub.getOrCreate(roomID, party.HostID.String())
	member := &wpMember{
		userID:   claims.UserID,
		username: claims.Username,
		send:     make(chan []byte, wsSendBuf),
	}
	room.addMember(member)

	// Send current state to the joining member
	pos, playing, memberList := room.snapshot()
	stateMsg, _ := json.Marshal(map[string]interface{}{
		"type":     "state",
		"position": pos,
		"playing":  playing,
		"members":  memberList,
	})
	member.send <- stateMsg

	// Broadcast updated member list to everyone (including new member)
	membersMsg, _ := json.Marshal(map[string]interface{}{
		"type":    "members",
		"members": memberList,
	})
	room.broadcast(membersMsg)

	// Write goroutine drains the send channel
	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range member.send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				conn.Close()
				return
			}
		}
	}()

	isHost := claims.UserID == room.hostID

	// Read loop
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if mt != websocket.TextMessage || !isHost {
			continue
		}
		var msg wpClientMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		var out []byte
		switch msg.Type {
		case "play":
			room.setState(msg.Position, true)
			out, _ = json.Marshal(map[string]interface{}{
				"type": "play", "position": msg.Position, "from": claims.Username,
			})
		case "pause":
			room.setState(msg.Position, false)
			out, _ = json.Marshal(map[string]interface{}{
				"type": "pause", "position": msg.Position, "from": claims.Username,
			})
		case "seek":
			room.setPosition(msg.Position)
			out, _ = json.Marshal(map[string]interface{}{
				"type": "seek", "position": msg.Position, "from": claims.Username,
			})
		default:
			continue
		}
		room.broadcast(out)
	}

	// Disconnect cleanup: safe to close send after removal because broadcast
	// can no longer reach this member once it's removed from the map.
	room.removeMember(claims.UserID)
	close(member.send)
	<-done

	if isHost {
		msg, _ := json.Marshal(map[string]string{"type": "closed"})
		room.broadcast(msg)
		h.hub.delete(roomID)
		_ = h.svc.Delete(claims.UserID, roomID) // ignore ErrNotFound if already deleted via HTTP
	} else {
		_, _, updatedList := room.snapshot()
		msg, _ := json.Marshal(map[string]interface{}{
			"type": "members", "members": updatedList,
		})
		room.broadcast(msg)
	}
}
