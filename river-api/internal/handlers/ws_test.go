package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"river-api/internal/middleware"
	"river-api/internal/models"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Dial harness ──────────────────────────────────────────────────────────────

func wsURL(srv *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http") + path
}

func dialWS(t *testing.T, srv *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL(srv, path), nil)
	require.NoError(t, err, "websocket handshake")
	if resp != nil {
		resp.Body.Close()
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	return conn
}

func readFrame(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := conn.ReadMessage()
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	return m
}

// readUntilType reads frames until one with the given "type" arrives (or fails).
func readUntilType(t *testing.T, conn *websocket.Conn, typ string) map[string]any {
	t.Helper()
	for i := 0; i < 12; i++ {
		m := readFrame(t, conn)
		if m["type"] == typ {
			return m
		}
	}
	t.Fatalf("did not receive a %q frame", typ)
	return nil
}

// readUntilMembersCount reads "members" frames until one lists exactly n members.
func readUntilMembersCount(t *testing.T, conn *websocket.Conn, n int) {
	t.Helper()
	for i := 0; i < 20; i++ {
		m := readFrame(t, conn)
		if m["type"] != "members" {
			continue
		}
		if arr, ok := m["members"].([]any); ok && len(arr) == n {
			return
		}
	}
	t.Fatalf("did not observe a members frame with %d members", n)
}

// ── progress WebSocket ──────────────────────────────────────────────────────────

func progressWSServer(repo *fakeProgressRepo) *httptest.Server {
	gin.SetMode(gin.TestMode)
	svc := services.NewProgressService(repo, &fakeMovieRepo{}, &fakeEpisodeRepo{}, &fakeSeasonRepo{}, &fakeShowRepo{}, &fakeUserRepo{}, &fakeAudiobookRepo{}, &fakeChapterRepo{}, &fakeDismissedRepo{})
	h := NewProgressWSHandler(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("claims", &middleware.Claims{UserID: c.Query("uid")}) })
	r.GET("/progress/ws", h.ServeWS)
	return httptest.NewServer(r)
}

func TestProgressWSHandler_ServeWS(t *testing.T) {
	ch := make(chan *models.WatchProgress, 8)
	srv := progressWSServer(&fakeProgressRepo{upserted: ch})
	defer srv.Close()

	conn := dialWS(t, srv, "/progress/ws?uid=u1")
	defer conn.Close()

	// Frames that must be ignored: binary, non-JSON, bad media type, empty id.
	require.NoError(t, conn.WriteMessage(websocket.BinaryMessage, []byte("binary")))
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`not json`)))
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"media_type":"photo","media_id":"x"}`)))
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"media_type":"movie","media_id":""}`)))
	// A valid frame is reported to the service.
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"media_type":"movie","media_id":"m1","position":30,"duration":100}`)))

	select {
	case got := <-ch:
		assert.Equal(t, "u1", got.UserID)
		assert.Equal(t, "movie", got.MediaType)
		assert.Equal(t, "m1", got.MediaID)
		assert.Equal(t, float64(30), got.Position)
	case <-time.After(2 * time.Second):
		t.Fatal("valid progress frame was not reported")
	}

	// The invalid frames (processed before the valid one) reported nothing.
	select {
	case extra := <-ch:
		t.Fatalf("unexpected extra report: %+v", extra)
	case <-time.After(150 * time.Millisecond):
	}
}

// ── watch-party WebSocket ───────────────────────────────────────────────────────

func watchPartyWSServer(repo *fakeWatchPartyRepo, hub *WatchPartyHub) *httptest.Server {
	gin.SetMode(gin.TestMode)
	h := NewWatchPartyHandler(services.NewWatchPartyService(repo), hub)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		uid := c.Query("uid")
		c.Set("claims", &middleware.Claims{UserID: uid, Username: "user-" + uid})
	})
	r.GET("/watchparty/:id/ws", h.ServeWS)
	return httptest.NewServer(r)
}

func seededParty(hostID uuid.UUID) *models.WatchParty {
	return &models.WatchParty{Base: models.Base{ID: uuid.New()}, HostID: hostID, MediaType: "movie", MediaID: "m1"}
}

func TestWatchPartyHandler_ServeWS_HostControls(t *testing.T) {
	hostID := uuid.New()
	party := seededParty(hostID)
	deleted := make(chan string, 1)
	srv := watchPartyWSServer(&fakeWatchPartyRepo{parties: []*models.WatchParty{party}, deleted: deleted}, NewWatchPartyHub())
	defer srv.Close()

	conn := dialWS(t, srv, "/watchparty/"+party.ID.String()+"/ws?uid="+hostID.String())

	// Joining member receives the current state and the member list.
	state := readUntilType(t, conn, "state")
	assert.Equal(t, false, state["playing"])
	readUntilType(t, conn, "members")

	// Host play / pause / seek control frames are echoed to the room.
	require.NoError(t, conn.WriteJSON(map[string]any{"type": "play", "position": 12.5}))
	play := readUntilType(t, conn, "play")
	assert.Equal(t, 12.5, play["position"])
	assert.Equal(t, "user-"+hostID.String(), play["from"])

	require.NoError(t, conn.WriteJSON(map[string]any{"type": "pause", "position": 20.0}))
	readUntilType(t, conn, "pause")

	// An unknown type hits the switch default and is silently dropped; the
	// following seek still comes back, proving the unknown one produced nothing.
	require.NoError(t, conn.WriteJSON(map[string]any{"type": "chat", "position": 0}))
	require.NoError(t, conn.WriteJSON(map[string]any{"type": "seek", "position": 30.0}))
	seek := readUntilType(t, conn, "seek")
	assert.Equal(t, 30.0, seek["position"])

	// Host disconnect closes the party (broadcast + hub delete + svc.Delete).
	conn.Close()
	select {
	case <-deleted:
	case <-time.After(2 * time.Second):
		t.Fatal("host disconnect did not delete the party")
	}
}

func TestWatchPartyHandler_ServeWS_MemberJoinLeave(t *testing.T) {
	hostID := uuid.New()
	party := seededParty(hostID)
	srv := watchPartyWSServer(&fakeWatchPartyRepo{parties: []*models.WatchParty{party}}, NewWatchPartyHub())
	defer srv.Close()

	base := "/watchparty/" + party.ID.String() + "/ws?uid="
	host := dialWS(t, srv, base+hostID.String())
	defer host.Close()
	readUntilType(t, host, "state")
	readUntilType(t, host, "members")

	// A non-host joins → the host sees the member list grow to 2.
	member := dialWS(t, srv, base+uuid.New().String())
	readUntilType(t, member, "state")
	readUntilMembersCount(t, host, 2)

	// A non-host's control frame is ignored (isHost == false → continue).
	require.NoError(t, member.WriteJSON(map[string]any{"type": "play", "position": 5}))

	// The non-host leaves → the host sees the list shrink back to 1.
	member.Close()
	readUntilMembersCount(t, host, 1)
}

func TestWatchPartyHandler_ServeWS_UnknownPartyIs404(t *testing.T) {
	srv := watchPartyWSServer(&fakeWatchPartyRepo{}, NewWatchPartyHub())
	defer srv.Close()

	// GetByID fails before the upgrade, so the handshake never completes.
	_, resp, err := websocket.DefaultDialer.Dial(wsURL(srv, "/watchparty/"+uuid.New().String()+"/ws?uid="+uuid.New().String()), nil)
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}
