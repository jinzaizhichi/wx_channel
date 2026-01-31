package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"wx_channel/hub_server/database"
)

func GetUserList(w http.ResponseWriter, r *http.Request) {
	// TODO: Auth check for Admin role

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}

	users, count, err := database.GetAllUsers(offset, limit)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total": count,
		"list":  users,
	})
}

func GetStats(w http.ResponseWriter, r *http.Request) {
	// TODO: Auth check for Admin role

	stats, err := database.GetSystemStats()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
