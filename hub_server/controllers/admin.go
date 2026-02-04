package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"wx_channel/hub_server/database"

	"github.com/gorilla/mux"
)

// GetUserList returns a list of all users (admin only)
func GetUserList(w http.ResponseWriter, r *http.Request) {
	users, _, err := database.GetAllUsers(0, 100)
	if err != nil {
		http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"list": users,
	})
}

// GetStats returns system statistics (admin only)
func GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := database.GetSystemStats()
	if err != nil {
		http.Error(w, "Failed to fetch stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// UpdateUserCredits adjusts a user's credits (admin only)
func UpdateUserCredits(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID     uint  `json:"user_id"`
		Adjustment int64 `json:"adjustment"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	if req.Adjustment == 0 {
		http.Error(w, "adjustment cannot be zero", http.StatusBadRequest)
		return
	}

	// 更新积分
	if err := database.AddCredits(req.UserID, req.Adjustment); err != nil {
		http.Error(w, "Failed to update credits", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Credits updated successfully",
	})
}

// UpdateUserRole changes a user's role (admin only)
func UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID uint   `json:"user_id"`
		Role   string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	if req.Role != "user" && req.Role != "admin" {
		http.Error(w, "role must be 'user' or 'admin'", http.StatusBadRequest)
		return
	}

	// 更新角色
	if err := database.UpdateUserRole(req.UserID, req.Role); err != nil {
		http.Error(w, "Failed to update role", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Role updated successfully",
	})
}

// DeleteUser permanently deletes a user (admin only)
func DeleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]

	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// 删除用户
	if err := database.DeleteUser(uint(userID)); err != nil {
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "User deleted successfully",
	})
}
