package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"wx_channel/hub_server/database"
	"wx_channel/hub_server/models"
	"wx_channel/hub_server/ws"
)

func GetTasks(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	nodeID := r.URL.Query().Get("node_id")

	userID := r.Context().Value("user_id").(uint)

	if limit <= 0 {
		limit = 20
	}

	tasks, count, err := database.GetTasks(userID, nodeID, offset, limit)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total": count,
		"list":  tasks,
	})
}

func GetTaskDetail(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}

	userID := r.Context().Value("user_id").(uint)
	task, err := database.GetTaskByID(uint(id), userID)
	if err != nil {
		http.Error(w, "Task not found", 404)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func RemoteCall(hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ClientID string          `json:"client_id"`
			Action   string          `json:"action"`
			Data     json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		// Check Credits
		userID := r.Context().Value("user_id").(uint)
		cost := int64(0)

		switch req.Action {
		case "search_channels", "search_videos":
			cost = 1
		case "download_video":
			cost = 10 // TODO: Dynamic cost based on resolution?
		}

		if cost > 0 {
			user, err := database.GetUserByID(userID)
			if err != nil {
				http.Error(w, "User not found", http.StatusNotFound)
				return
			}

			if user.Credits < cost {
				http.Error(w, "Insufficient credits", http.StatusPaymentRequired) // 402
				return
			}

			// Deduct credits
			if err := database.AddCredits(userID, -cost); err != nil {
				http.Error(w, "Transaction failed", http.StatusInternalServerError)
				return
			}

			// Record Transaction
			database.RecordTransaction(&models.Transaction{
				UserID:      userID,
				Amount:      -cost,
				Type:        req.Action,
				Description: "API Call: " + req.Action,
				RelatedID:   req.ClientID,
				CreatedAt:   time.Now(),
			})
		}

		resp, err := hub.Call(userID, req.ClientID, req.Action, req.Data, 30*time.Second)
		if err != nil {
			// Refund on failure?
			// For search, maybe not. For download, yes.
			// Currently, we charge upfront.
			http.Error(w, err.Error(), 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
