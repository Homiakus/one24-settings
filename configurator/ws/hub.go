package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"modbus-configurator/model"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Hub управляет WebSocket-подключениями и рассылкой событий.
type Hub struct {
	mu             sync.RWMutex
	clients        map[*client]struct{}
	maxClients     int
	pendingClients int
}

// NewHub создаёт новый хаб.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*client]struct{}),
		maxClients: 0, // без ограничений
	}
}

// SetMaxClients устанавливает лимит подключений (0 = без лимита).
func (h *Hub) SetMaxClients(n int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.maxClients = n
}

type client struct {
	conn   *websocket.Conn
	send   chan []byte
	topics map[string]bool
}

// reserveClientSlot атомарно учитывает ещё не завершившийся WebSocket handshake.
// Без резервирования Dial может получить 101 Switching Protocols раньше, чем
// ServeHTTP добавит соединение в clients, и несколько запросов превысят лимит.
func (h *Hub) reserveClientSlot() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.maxClients > 0 && len(h.clients)+h.pendingClients >= h.maxClients {
		return false
	}
	h.pendingClients++
	return true
}

func (h *Hub) releaseClientReservation() {
	h.mu.Lock()
	if h.pendingClients > 0 {
		h.pendingClients--
	}
	h.mu.Unlock()
}

// ServeHTTP обрабатывает WebSocket-подключение.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.reserveClientSlot() {
		http.Error(w, "слишком много WebSocket-подключений", http.StatusServiceUnavailable)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.releaseClientReservation()
		log.Printf("[ws] upgrade error: %v", err)
		return
	}

	topics := make(map[string]bool)
	if q := r.URL.Query().Get("topics"); q != "" {
		for _, t := range splitTopics(q) {
			topics[t] = true
		}
	}
	if len(topics) == 0 {
		for _, t := range []string{"status", "progress", "sensors", "reagent", "error", "log", "disconnect", "connect"} {
			topics[t] = true
		}
	}

	c := &client{
		conn:   conn,
		send:   make(chan []byte, 64),
		topics: topics,
	}

	h.mu.Lock()
	if h.pendingClients > 0 {
		h.pendingClients--
	}
	h.clients[c] = struct{}{}
	total := len(h.clients)
	h.mu.Unlock()

	log.Printf("[ws] client connected (topics=%v, total=%d)", topicKeys(topics), total)

	go c.writePump()
	c.readPump(h, c)
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	total := len(h.clients)
	h.mu.Unlock()
	log.Printf("[ws] client disconnected (total=%d)", total)
}

// Broadcast отправляет событие всем подписанным клиентам.
func (h *Hub) Broadcast(event model.WSEvent) {
	event.Timestamp = time.Now()
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[ws] marshal error: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients {
		if c.topics[event.Type] || c.topics["*"] {
			select {
			case c.send <- data:
			default:
			}
		}
	}
}

func (c *client) readPump(h *Hub, cl *client) {
	defer func() {
		h.remove(cl)
		cl.conn.Close()
	}()

	cl.conn.SetReadLimit(512)
	cl.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	cl.conn.SetPongHandler(func(string) error {
		cl.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := cl.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func splitTopics(s string) []string {
	var result []string
	cur := ""
	for _, ch := range s {
		if ch == ',' {
			if cur != "" {
				result = append(result, cur)
				cur = ""
			}
		} else {
			cur += string(ch)
		}
	}
	if cur != "" {
		result = append(result, cur)
	}
	return result
}

func topicKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
