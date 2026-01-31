package main

import (
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"wx_channel/hub_server/controllers"
	"wx_channel/hub_server/database"
	"wx_channel/hub_server/middleware"
	"wx_channel/hub_server/services"
	"wx_channel/hub_server/ws"
)

func main() {
	// 1. 初始化数据库
	if err := database.InitDB("hub_server.db"); err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	// 2. 初始化 WebSocket Hub
	hub := ws.NewHub()
	go hub.Run()

	// 2.5 启动积分矿工服务 (在线时长统计)
	services.StartMiningService()

	// 3. Middleware: Panic Recovery
	withRecovery := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("PANIC: %v\nStack: %s", err, string(debug.Stack()))
					http.Error(w, "Internal Server Error", 500)
				}
			}()
			next(w, r)
		}
	}

	// 4. 注册路由

	// WebSocket 接入点
	http.HandleFunc("/ws/client", withRecovery(hub.ServeWs))

	// Auth API
	http.HandleFunc("/api/auth/register", withRecovery(controllers.Register))
	http.HandleFunc("/api/auth/login", withRecovery(controllers.Login))

	// Protected API (Need Auth)
	http.HandleFunc("/api/auth/profile", withRecovery(middleware.AuthRequired(controllers.GetProfile)))
	http.HandleFunc("/api/device/bind_token", withRecovery(middleware.AuthRequired(controllers.GenerateBindToken)))
	http.HandleFunc("/api/device/list", withRecovery(middleware.AuthRequired(controllers.GetUserDevices)))

	// Admin API
	http.HandleFunc("/api/admin/users", withRecovery(middleware.AuthRequired(middleware.AdminRequired(controllers.GetUserList))))
	http.HandleFunc("/api/admin/stats", withRecovery(middleware.AuthRequired(middleware.AdminRequired(controllers.GetStats))))

	// Public API (For now, keeping them public or applying OptionalAuth as needed)
	// 在未来阶段，这些应该加上 AuthRequired
	http.HandleFunc("/api/clients", withRecovery(controllers.GetNodes))

	http.HandleFunc("/api/tasks", withRecovery(middleware.AuthRequired(controllers.GetTasks)))
	http.HandleFunc("/api/tasks/detail", withRecovery(middleware.AuthRequired(controllers.GetTaskDetail)))
	http.HandleFunc("/api/call", withRecovery(middleware.AuthRequired(controllers.RemoteCall(hub))))

	// Video Play
	http.HandleFunc("/api/video/play", withRecovery(controllers.PlayVideo))

	// 静态文件服务 - Vue SPA 支持
	// 优先服务 frontend/dist 目录下的静态资源
	fs := http.FileServer(http.Dir("frontend/dist"))
	http.HandleFunc("/", withRecovery(func(w http.ResponseWriter, r *http.Request) {
		// 如果是 API 调用或 WebSocket，不处理
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/") {
			return
		}

		path := r.URL.Path
		// 检查文件是否存在于 dist 目录
		if _, err := os.Stat("frontend/dist" + path); os.IsNotExist(err) {
			// 文件不存在，返回 index.html (SPA History Mode)
			http.ServeFile(w, r, "frontend/dist/index.html")
			return
		}

		// 文件存在，直接服务
		fs.ServeHTTP(w, r)
	}))

	log.Println("Hub Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
