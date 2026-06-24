package ws

import (
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	mu    sync.RWMutex
	conns map[int64]map[*websocket.Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{conns: make(map[int64]map[*websocket.Conn]struct{})}
}

func (h *Hub) Register(userID int64, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.conns[userID]; !ok {
		h.conns[userID] = make(map[*websocket.Conn]struct{})
	}
	h.conns[userID][conn] = struct{}{}
}

func (h *Hub) Unregister(userID int64, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.conns[userID]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(h.conns, userID)
		}
	}
}

func (h *Hub) HasUser(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conns, ok := h.conns[userID]
	return ok && len(conns) > 0
}

func (h *Hub) SendToUser(userID int64, msg []byte) {
	h.mu.RLock()
	conns := h.conns[userID]
	h.mu.RUnlock()
	if conns == nil {
		return
	}
	// send in goroutines to avoid blocking
	for c := range conns {
		go func(conn *websocket.Conn) {
			_ = conn.WriteMessage(websocket.TextMessage, msg)
		}(c)
	}
}
