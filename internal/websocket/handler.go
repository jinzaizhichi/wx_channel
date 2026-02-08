package websocket

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/coder/websocket"
)

// Handler WebSocket HTTP 处理器
type Handler struct {
	hub            *Hub
	allowedOrigins []string
	secretToken    string
}

// NewHandler 创建新的处理器
func NewHandler(hub *Hub, allowedOrigins []string, secretToken string) *Handler {
	return &Handler{
		hub:            hub,
		allowedOrigins: allowedOrigins,
		secretToken:    secretToken,
	}
}

// ServeHTTP 处理 WebSocket 连接请求
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if !isOriginAllowed(origin, h.allowedOrigins) {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}

	if h.secretToken != "" {
		token := extractTokenFromRequest(r)
		if token != h.secretToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionContextTakeover,
	})
	if err != nil {
		fmt.Printf("[WebSocket] 连接升级失败: %v\n", err)
		http.Error(w, "Could not open websocket connection", http.StatusBadRequest)
		return
	}

	// 获取远程地址
	remoteAddr := r.RemoteAddr
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		remoteAddr = realIP
	} else if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		remoteAddr = forwarded
	}

	client := NewClientWithAddr(conn, h.hub, remoteAddr)
	h.hub.RegisterClient(client)

	// 启动读写协程
	go client.WritePump()
	go client.ReadPump()
}

func isOriginAllowed(origin string, allowedOrigins []string) bool {
	if len(allowedOrigins) == 0 {
		return true
	}
	if origin == "" {
		return false
	}
	for _, o := range allowedOrigins {
		if o == "*" || o == origin {
			return true
		}
	}
	return false
}

func extractTokenFromRequest(r *http.Request) string {
	token := strings.TrimSpace(r.Header.Get("X-Local-Auth"))
	if token != "" {
		return token
	}

	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("Bearer "):])
	}

	return strings.TrimSpace(r.URL.Query().Get("token"))
}
