package handler

import (
	"backend/pkg/db/sqlite"
	"backend/pkg/model"
	"backend/pkg/repository"
	"backend/util"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var logData model.LoginData

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&logData)
	if err != nil {
		http.Error(w, "Error parsing login JSON data: "+err.Error(), http.StatusBadRequest)
		return
	}
	user, err := repository.GetUserByEmailOrNickname(sqlite.Dbase, logData.Username)
	if err != nil {
		fmt.Printf("Error getting user by email of nickname: %v <> error: %v \n", logData.Username, err)
		return
	}

	if user.Id == 0 {
		// User not found
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	// Compare hashed password with provided password

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(logData.Password))
	if err != nil { // Wrong password
		fmt.Println("Error comparing password: ", err)
		http.Error(w, "Couldn't compare password with hashed variant: "+ err.Error(), http.StatusUnauthorized)
		return
	}

	// Generate a session token and store it in database
	sessionToken := util.GenerateSessionToken()
	repository.StoreSessionInDB(sqlite.Dbase, sessionToken, user.Id)
	
	// Set a cookie with the session
	http.SetCookie(w, &http.Cookie{
		Name: "session_token",
		Value: sessionToken,
		MaxAge: 60*15, // 15 minutes
	})

	// Send a success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Login successful",
	})
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	_, err := r.Cookie("session_token")
	if err != nil {
		if err == http.ErrNoCookie {
			http.Error(w, "User already logged out", http.StatusBadRequest)
			return
		}
		// For any other error, return a bad request status
		http.Error(w, "Bad request: "+ err.Error(), http.StatusBadRequest)
		return
	}

	// Delete the session-token cookie
	http.SetCookie(w, &http.Cookie{
		Name: "session_token",
		Value: "",
		MaxAge: -1, // Setting MaxAge to -1 immediately expires the cookie
		Expires: time.Unix(0, 0),
	})

	// Send a success reponse
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Logout successful",
	})
}

func CheckAuth(w http.ResponseWriter, r *http.Request) {
	isAuthenticated := true

	cookie, err := r.Cookie("session_token")
	if err != nil {
		if err == http.ErrNoCookie {
			// If the session cookie doesn't exist, set isAuthenticated to false
			isAuthenticated = false
		} else {
			http.Error(w, "Error checking session token: " + err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if isAuthenticated {
		sessionToken := cookie.Value

		// Get the session from database by the session token
		session, err := repository.GetSessionBySessionToken(sqlite.Dbase, sessionToken)
		if err != nil {
			if err == sql.ErrNoRows {
				isAuthenticated = false
			} else {
				fmt.Println("Error getting session token from database: " + err.Error())
				return
			}
		}
		if time.Now().After(session.ExpiresAt) {
			isAuthenticated = false
		}
	}
	response := model.AuthResponse{
		IsAuthenticated: isAuthenticated,
	}
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Error marshalling response: " + err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResponse)
}