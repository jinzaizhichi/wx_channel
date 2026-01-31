package ws

import (
	"encoding/json"
	"sync"
	"time"
	"wx_channel/hub_server/database"
	"wx_channel/hub_server/models"
	"wx_channel/hub_server/services"

	"github.com/gorilla/websocket"
)

type Client struct {
	ID       string
	Hostname string
	Version  string
	LastSeen time.Time
	Conn     *websocket.Conn
	mu       sync.Mutex

	respChannels map[string]chan ResponsePayload
	respMu       sync.RWMutex
	Hub          *Hub
}

func NewClient(id string, conn *websocket.Conn, hub *Hub) *Client {
	return &Client{
		ID:           id,
		LastSeen:     time.Now(),
		Conn:         conn,
		respChannels: make(map[string]chan ResponsePayload),
		Hub:          hub,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var msg CloudMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		c.handleMessage(msg)
	}
}

func (c *Client) handleMessage(msg CloudMessage) {
	c.mu.Lock()
	c.LastSeen = time.Now()
	c.mu.Unlock()

	switch msg.Type {
	case MsgTypeHeartbeat:
		var p HeartbeatPayload
		json.Unmarshal(msg.Payload, &p)
		c.mu.Lock()
		c.Hostname = p.Hostname
		c.Version = p.Version
		c.mu.Unlock()

		// DB: Update heartbeat info
		database.UpsertNode(&models.Node{
			ID:       c.ID,
			Hostname: p.Hostname,
			Version:  p.Version,
			Status:   "online",
			LastSeen: time.Now(),
		})

	case MsgTypeResponse:
		var resp ResponsePayload
		if err := json.Unmarshal(msg.Payload, &resp); err == nil {
			c.respMu.RLock()
			ch, ok := c.respChannels[resp.RequestID]
			c.respMu.RUnlock()
			if ok {
				ch <- resp
			}
		}

	case MsgTypeBind:
		var payload struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err == nil {
			err := services.ProcessBindRequest(c.ID, payload.Token)

			// Notify client of result
			response := map[string]interface{}{
				"type":    "bind_result",
				"success": err == nil,
			}
			if err != nil {
				response["error"] = err.Error()
			}

			respBytes, _ := json.Marshal(response)
			c.WriteMessage(respBytes)
		}
	}
}

func (c *Client) WriteMessage(msg []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteMessage(websocket.TextMessage, msg)
}
