package controllers

import (
	"encoding/json"
	"net/http"

	"wx_channel/hub_server/database"
	"wx_channel/hub_server/services"
)

// GenerateBindToken returns a short code for the user to input in the client
func GenerateBindToken(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(uint)

	token, err := services.Binder.GenerateToken(userID)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": token,
	})
}

// GetUserDevices returns all devices bound to the current user
func GetUserDevices(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(uint)

	user, err := database.GetUserByID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user.Devices)
}

// UnbindDevice removes the binding between a device and the user
func UnbindDevice(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement unbinding logic if needed
	// 1. Get NodeID from request
	// 2. Check if Node belongs to UserID
	// 3. Set Node.UserID = 0, BindStatus = false
	w.WriteHeader(http.StatusNotImplemented)
}
