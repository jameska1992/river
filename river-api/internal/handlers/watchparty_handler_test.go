package handlers

import (
	"net/http"
	"testing"

	"river-api/internal/apperrors"
	"river-api/internal/middleware"
	"river-api/internal/models"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fake ---

type fakeWatchPartyRepo struct {
	parties   []*models.WatchParty
	createErr error
}

func (f *fakeWatchPartyRepo) Create(p *models.WatchParty) error {
	if f.createErr != nil {
		return f.createErr
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	f.parties = append(f.parties, p)
	return nil
}
func (f *fakeWatchPartyRepo) FindByID(id string) (*models.WatchParty, error) {
	for _, p := range f.parties {
		if p.ID.String() == id {
			return p, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (f *fakeWatchPartyRepo) Delete(id string) error {
	for i, p := range f.parties {
		if p.ID.String() == id {
			f.parties = append(f.parties[:i], f.parties[i+1:]...)
			return nil
		}
	}
	return apperrors.ErrNotFound
}

// --- progress_ws small helper ---

func TestIsValidProgressMediaType(t *testing.T) {
	for _, ok := range []string{"movie", "episode", "chapter"} {
		assert.True(t, isValidProgressMediaType(ok), ok)
	}
	for _, bad := range []string{"", "photo", "Movie", "book"} {
		assert.False(t, isValidProgressMediaType(bad), bad)
	}
}

// --- in-memory hub / room ---

func TestWatchPartyHub_RoomLifecycle(t *testing.T) {
	hub := NewWatchPartyHub()

	r1 := hub.getOrCreate("room1", "host1")
	r2 := hub.getOrCreate("room1", "someone-else")
	assert.Same(t, r1, r2, "getOrCreate returns the existing room and keeps the original host")
	assert.Equal(t, "host1", r1.hostID)

	hub.delete("room1")
	r3 := hub.getOrCreate("room1", "host2")
	assert.NotSame(t, r1, r3, "after delete a fresh room is created")
	assert.Equal(t, "host2", r3.hostID)
}

func TestWatchPartyRoom_MembersAndState(t *testing.T) {
	r := NewWatchPartyHub().getOrCreate("room", "host")

	m := &wpMember{userID: "u1", username: "alice", send: make(chan []byte, wsSendBuf)}
	r.addMember(m)

	pos, playing, list := r.snapshot()
	assert.Equal(t, float64(0), pos)
	assert.False(t, playing)
	require.Len(t, list, 1)
	assert.Equal(t, "alice", list[0]["username"])

	r.setState(42.5, true)
	pos, playing, _ = r.snapshot()
	assert.Equal(t, 42.5, pos)
	assert.True(t, playing)

	r.setPosition(99)
	pos, _, _ = r.snapshot()
	assert.Equal(t, float64(99), pos)

	// broadcast reaches the member's buffered channel.
	r.broadcast([]byte("hello"))
	assert.Equal(t, "hello", string(<-m.send))

	r.removeMember("u1")
	_, _, list = r.snapshot()
	assert.Empty(t, list)
}

func TestWatchPartyRoom_BroadcastDropsWhenBufferFull(t *testing.T) {
	r := NewWatchPartyHub().getOrCreate("room", "host")
	// Buffer size 1, never drained → the second broadcast must not block.
	m := &wpMember{userID: "u1", send: make(chan []byte, 1)}
	r.addMember(m)
	r.broadcast([]byte("a"))
	r.broadcast([]byte("b")) // dropped via the select-default, no deadlock
	assert.Equal(t, "a", string(<-m.send))
}

// --- HTTP handlers ---

func watchPartyRouter(repo *fakeWatchPartyRepo, hub *WatchPartyHub, callerID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewWatchPartyHandler(services.NewWatchPartyService(repo), hub)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("claims", &middleware.Claims{UserID: callerID, Username: "caller"}) })
	r.POST("/watchparty", h.Create)
	r.GET("/watchparty/:id", h.Get)
	r.DELETE("/watchparty/:id", h.Delete)
	return r
}

func TestWatchPartyHandler_Create(t *testing.T) {
	host := uuid.New().String()

	t.Run("valid is 201", func(t *testing.T) {
		repo := &fakeWatchPartyRepo{}
		r := watchPartyRouter(repo, NewWatchPartyHub(), host)
		w := doJSON(r, http.MethodPost, "/watchparty", `{"media_type":"movie","media_id":"m1"}`)
		require.Equal(t, http.StatusCreated, w.Code)
		assert.Len(t, repo.parties, 1)
	})
	t.Run("missing fields is 400", func(t *testing.T) {
		r := watchPartyRouter(&fakeWatchPartyRepo{}, NewWatchPartyHub(), host)
		w := doJSON(r, http.MethodPost, "/watchparty", `{"media_type":"movie"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("non-uuid host is 401", func(t *testing.T) {
		r := watchPartyRouter(&fakeWatchPartyRepo{}, NewWatchPartyHub(), "not-a-uuid")
		w := doJSON(r, http.MethodPost, "/watchparty", `{"media_type":"movie","media_id":"m1"}`)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestWatchPartyHandler_Get(t *testing.T) {
	party := &models.WatchParty{Base: models.Base{ID: uuid.New()}, HostID: uuid.New(), MediaType: "movie", MediaID: "m1"}
	r := watchPartyRouter(&fakeWatchPartyRepo{parties: []*models.WatchParty{party}}, NewWatchPartyHub(), uuid.New().String())

	t.Run("found is 200", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, doJSON(r, http.MethodGet, "/watchparty/"+party.ID.String(), "").Code)
	})
	t.Run("missing is 404", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, doJSON(r, http.MethodGet, "/watchparty/"+uuid.New().String(), "").Code)
	})
}

func TestWatchPartyHandler_Delete(t *testing.T) {
	hostID := uuid.New()

	t.Run("host deletes (204) and clears the room", func(t *testing.T) {
		party := &models.WatchParty{Base: models.Base{ID: uuid.New()}, HostID: hostID, MediaType: "movie", MediaID: "m1"}
		repo := &fakeWatchPartyRepo{parties: []*models.WatchParty{party}}
		hub := NewWatchPartyHub()
		// A live room with a member so the notify-then-delete branch runs.
		room := hub.getOrCreate(party.ID.String(), hostID.String())
		room.addMember(&wpMember{userID: "u1", send: make(chan []byte, wsSendBuf)})

		r := watchPartyRouter(repo, hub, hostID.String())
		w := doJSON(r, http.MethodDelete, "/watchparty/"+party.ID.String(), "")
		require.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, repo.parties)
		assert.Nil(t, hub.rooms[party.ID.String()], "room removed from the hub")
	})

	t.Run("non-host is 401", func(t *testing.T) {
		party := &models.WatchParty{Base: models.Base{ID: uuid.New()}, HostID: hostID, MediaType: "movie", MediaID: "m1"}
		repo := &fakeWatchPartyRepo{parties: []*models.WatchParty{party}}
		r := watchPartyRouter(repo, NewWatchPartyHub(), uuid.New().String())
		w := doJSON(r, http.MethodDelete, "/watchparty/"+party.ID.String(), "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("unknown party is 404", func(t *testing.T) {
		r := watchPartyRouter(&fakeWatchPartyRepo{}, NewWatchPartyHub(), hostID.String())
		w := doJSON(r, http.MethodDelete, "/watchparty/"+uuid.New().String(), "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
