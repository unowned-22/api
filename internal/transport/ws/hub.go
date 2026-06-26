package ws

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/unowned-22/api/internal/domain/friendship"
	"github.com/unowned-22/api/internal/domain/messenger"
	"github.com/unowned-22/api/internal/logger"
)

// writeTimeout bounds how long a single WS write may block, so a slow or
// dead peer cannot leak goroutines indefinitely.
const writeTimeout = 10 * time.Second

// presenceLockTTL is how long a per-user presence mutex is kept in the map
// after it was last used. Entries older than this are swept by the background
// cleaner when the user has no active connections.
const presenceLockTTL = 30 * time.Minute

type Hub struct {
	mu    sync.RWMutex
	conns map[int64]map[*websocket.Conn]*sync.Mutex // conn -> dedicated write-mutex

	presenceRepo messenger.PresenceRepository
	friendRepo   friendship.Repository

	// presenceLocks serializes onFirstConnect/onLastDisconnect per user so a
	// stale disconnect callback can never "win" a race against a more recent
	// reconnect (and vice versa). Guarded by presenceMu.
	//
	// presenceLockLastUsed tracks when each entry was last touched so the
	// background sweeper can evict idle entries and bound map growth for
	// long-running processes with millions of unique users.
	presenceMu           sync.Mutex
	presenceLocks        map[int64]*sync.Mutex
	presenceLockLastUsed map[int64]time.Time
}

func NewHub() *Hub {
	h := &Hub{
		conns:                make(map[int64]map[*websocket.Conn]*sync.Mutex),
		presenceLocks:        make(map[int64]*sync.Mutex),
		presenceLockLastUsed: make(map[int64]time.Time),
	}
	go h.sweepPresenceLocks()
	return h
}

// NewHubWithPresence creates a Hub wired with presence and friend repositories.
// This variant is used in production so that online/offline events are broadcast
// to friends when a user connects or disconnects.
func NewHubWithPresence(presenceRepo messenger.PresenceRepository, friendRepo friendship.Repository) *Hub {
	h := &Hub{
		conns:                make(map[int64]map[*websocket.Conn]*sync.Mutex),
		presenceRepo:         presenceRepo,
		friendRepo:           friendRepo,
		presenceLocks:        make(map[int64]*sync.Mutex),
		presenceLockLastUsed: make(map[int64]time.Time),
	}
	go h.sweepPresenceLocks()
	return h
}

// presenceLockFor returns a per-user mutex, creating it on first use.
// It records the current time so the background sweeper can evict entries
// that haven't been touched for presenceLockTTL and have no active connections.
func (h *Hub) presenceLockFor(userID int64) *sync.Mutex {
	h.presenceMu.Lock()
	defer h.presenceMu.Unlock()
	lock, ok := h.presenceLocks[userID]
	if !ok {
		lock = &sync.Mutex{}
		h.presenceLocks[userID] = lock
	}
	h.presenceLockLastUsed[userID] = time.Now()
	return lock
}

// sweepPresenceLocks runs in the background and periodically removes entries
// from presenceLocks that are both idle (last used > presenceLockTTL ago) and
// have no active connections. This bounds map growth for long-running processes
// serving large numbers of unique users without restarting.
//
// Safety: we only delete an entry when the user has no active connections,
// which means neither onFirstConnect nor onLastDisconnect can be mid-flight
// for that user (both are triggered by Register/Unregister, which update h.conns
// under h.mu before the goroutine is launched). Deleting the map entry while
// the mutex itself is NOT held by anyone is therefore safe.
//
// Lock order: presenceMu → mu (via HasUser). This order must be maintained
// everywhere in Hub to prevent deadlocks. No code path acquires mu first and
// then presenceMu.
func (h *Hub) sweepPresenceLocks() {
	ticker := time.NewTicker(presenceLockTTL / 2)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-presenceLockTTL)
		h.presenceMu.Lock()
		for userID, lastUsed := range h.presenceLockLastUsed {
			if lastUsed.Before(cutoff) && !h.HasUser(userID) {
				delete(h.presenceLocks, userID)
				delete(h.presenceLockLastUsed, userID)
			}
		}
		h.presenceMu.Unlock()
	}
}

func (h *Hub) Register(userID int64, conn *websocket.Conn) {
	h.mu.Lock()
	if _, ok := h.conns[userID]; !ok {
		h.conns[userID] = make(map[*websocket.Conn]*sync.Mutex)
	}
	isFirst := len(h.conns[userID]) == 0
	h.conns[userID][conn] = &sync.Mutex{}
	h.mu.Unlock()

	if isFirst && h.presenceRepo != nil {
		go h.onFirstConnect(userID)
	}
}

func (h *Hub) Unregister(userID int64, conn *websocket.Conn) {
	h.mu.Lock()
	isLast := false
	if conns, ok := h.conns[userID]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(h.conns, userID)
			isLast = true
		}
	}
	h.mu.Unlock()

	if isLast && h.presenceRepo != nil {
		go h.onLastDisconnect(userID)
	}
}

// onFirstConnect announces the user as online to their friends.
//
// It is launched in its own goroutine by Register, so a rapid
// disconnect/reconnect can in theory cause it to run out of order relative
// to a matching onLastDisconnect call for the same user. The per-user
// presenceLock plus a fresh HasUser check (instead of trusting the snapshot
// taken at schedule time) makes the pair of callbacks consistent regardless
// of goroutine scheduling order.
func (h *Hub) onFirstConnect(userID int64) {
	lock := h.presenceLockFor(userID)
	lock.Lock()
	defer lock.Unlock()

	if !h.HasUser(userID) {
		// The user already disconnected again before this callback got to
		// run — nothing to announce, and onLastDisconnect (if it also runs)
		// will see no connections and proceed normally.
		return
	}

	ctx := context.Background()
	if err := h.presenceRepo.SetOnline(ctx, userID); err != nil {
		logger.Log.WithError(err).Warnf("Hub.onFirstConnect: failed to set user %d online", userID)
	}

	if h.friendRepo == nil {
		return
	}
	friendIDs, err := h.friendRepo.GetFriendIDs(ctx, userID)
	if err != nil {
		logger.Log.WithError(err).Warnf("Hub.onFirstConnect: failed to get friend IDs for user %d", userID)
		return
	}
	for _, fid := range friendIDs {
		if err := SendMessengerEvent(h, fid, "messenger.presence", MessengerPresencePayload{
			UserID:   userID,
			IsOnline: true,
		}); err != nil {
			logger.Log.WithError(err).Warnf("Hub.onFirstConnect: failed to push presence to user %d", fid)
		}
	}
}

// onLastDisconnect announces the user as offline to their friends.
//
// See onFirstConnect for why the presenceLock + re-check of the live
// connection state is required: without it, a disconnect callback that gets
// scheduled late could mark a user offline even though they already
// reconnected.
func (h *Hub) onLastDisconnect(userID int64) {
	lock := h.presenceLockFor(userID)
	lock.Lock()
	defer lock.Unlock()

	if h.HasUser(userID) {
		// The user reconnected before this stale callback got to run —
		// they are still online, so there is nothing to broadcast.
		return
	}

	ctx := context.Background()
	if err := h.presenceRepo.SetOffline(ctx, userID); err != nil {
		logger.Log.WithError(err).Errorf(
			"Hub.onLastDisconnect: failed to set user %d offline, skipping presence broadcast", userID,
		)
		return // не рассылаем, пока БД не зафиксировала оффлайн
	}

	presence, err := h.presenceRepo.GetPresence(ctx, userID)
	if err != nil {
		logger.Log.WithError(err).Warnf(
			"Hub.onLastDisconnect: failed to get presence for user %d, broadcasting without last_seen_at", userID,
		)
		// продолжаем — last_seen_at будет nil, это допустимо
	}

	if h.friendRepo == nil {
		return
	}
	friendIDs, err := h.friendRepo.GetFriendIDs(ctx, userID)
	if err != nil {
		logger.Log.WithError(err).Warnf("Hub.onLastDisconnect: failed to get friend IDs for user %d", userID)
		return
	}
	payload := MessengerPresencePayload{UserID: userID, IsOnline: false}
	if presence != nil {
		t := presence.LastSeenAt
		payload.LastSeenAt = &t
	}
	for _, fid := range friendIDs {
		if err := SendMessengerEvent(h, fid, "messenger.presence", payload); err != nil {
			logger.Log.WithError(err).Warnf("Hub.onLastDisconnect: failed to push presence to user %d", fid)
		}
	}
}

func (h *Hub) HasUser(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conns, ok := h.conns[userID]
	return ok && len(conns) > 0
}

// SendToUser fans msg out to every connection currently open for userID.
//
// Each connection has a dedicated write-mutex (set up in Register) which is
// held for the duration of the write. gorilla/websocket forbids concurrent
// writers on the same *websocket.Conn; without this lock, two events for the
// same user arriving close together (e.g. a message + a presence update)
// could race on the same connection and corrupt the WS stream or panic.
func (h *Hub) SendToUser(userID int64, msg []byte) {
	h.mu.RLock()
	conns := h.conns[userID]
	targets := make([]*websocket.Conn, 0, len(conns))
	locks := make([]*sync.Mutex, 0, len(conns))
	for c, writeMu := range conns {
		targets = append(targets, c)
		locks = append(locks, writeMu)
	}
	h.mu.RUnlock()

	for i, conn := range targets {
		writeMu := locks[i]
		go func(conn *websocket.Conn, writeMu *sync.Mutex) {
			writeMu.Lock()
			defer writeMu.Unlock()
			_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			_ = conn.WriteMessage(websocket.TextMessage, msg)
		}(conn, writeMu)
	}
}
