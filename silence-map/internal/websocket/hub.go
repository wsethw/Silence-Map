package websocket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	gws "github.com/gorilla/websocket"
	"github.com/portfolio/silence-map/internal/domain"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 2048
)

type Hub struct {
	register   chan *Client
	unregister chan *Client
	broadcast  chan Event
	clients    map[*Client]struct{}
	upgrader   gws.Upgrader
}

type Event struct {
	Type          string         `json:"type"`
	Report        *domain.Report `json:"report,omitempty"`
	ReportID      string         `json:"report_id,omitempty"`
	Quietness     int            `json:"quietness,omitempty"`
	Confirmations int            `json:"confirmations,omitempty"`
	Location      *domain.Point  `json:"location,omitempty"`
}

type Client struct {
	hub        *Hub
	conn       *gws.Conn
	send       chan []byte
	mu         sync.RWMutex
	bounds     domain.Bounds
	subscribed bool
}

type clientMessage struct {
	Action string        `json:"action"`
	Bounds domain.Bounds `json:"bounds"`
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan Event, 512),
		clients:    make(map[*Client]struct{}),
		upgrader: gws.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" || origin == "null" {
					return true
				}
				originURL, err := url.Parse(origin)
				return err == nil && originURL.Host == r.Host
			},
		},
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = struct{}{}
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case event := <-h.broadcast:
			payload, err := json.Marshal(event)
			if err != nil {
				log.Printf("websocket: marshal event: %v", err)
				continue
			}
			for client := range h.clients {
				if !client.accepts(eventPoint(event)) {
					continue
				}
				select {
				case client.send <- payload:
				default:
					delete(h.clients, client)
					close(client.send)
				}
			}
		}
	}
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket: upgrade: %v", err)
		return
	}

	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

func (h *Hub) PublishReport(ctx context.Context, report domain.Report) {
	point := report.Location
	event := Event{
		Type:     "new_report",
		Report:   &report,
		Location: &point,
	}
	h.publish(ctx, event)
}

func (h *Hub) PublishConfirmation(ctx context.Context, report domain.Report) {
	point := report.Location
	event := Event{
		Type:          "confirmation",
		Report:        &report,
		ReportID:      report.ID,
		Quietness:     report.QuietnessLevel,
		Confirmations: report.ConfirmationCount,
		Location:      &point,
	}
	h.publish(ctx, event)
}

func (h *Hub) publish(ctx context.Context, event Event) {
	select {
	case h.broadcast <- event:
	case <-ctx.Done():
	default:
		log.Printf("websocket: dropping %s event because broadcast buffer is full", event.Type)
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, payload, err := c.conn.ReadMessage()
		if err != nil {
			if gws.IsUnexpectedCloseError(err, gws.CloseGoingAway, gws.CloseAbnormalClosure) {
				log.Printf("websocket: read: %v", err)
			}
			return
		}

		var msg clientMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			c.queueError("invalid_json", "message must be valid JSON")
			continue
		}

		if msg.Action == "subscribe" && msg.Bounds.Valid() {
			c.mu.Lock()
			c.bounds = msg.Bounds
			c.subscribed = true
			c.mu.Unlock()
			continue
		}

		c.queueError("invalid_subscription", "expected subscribe action with valid bounds")
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(gws.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(gws.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(gws.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) accepts(point *domain.Point) bool {
	if point == nil {
		return false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.subscribed && c.bounds.Contains(*point)
}

func eventPoint(event Event) *domain.Point {
	if event.Location != nil {
		return event.Location
	}
	if event.Report != nil {
		return &event.Report.Location
	}
	return nil
}

func (c *Client) queueError(code, message string) {
	payload, err := json.Marshal(map[string]string{
		"type":    "error",
		"code":    code,
		"message": message,
	})
	if err != nil {
		return
	}

	select {
	case c.send <- payload:
	default:
		c.hub.unregister <- c
	}
}
