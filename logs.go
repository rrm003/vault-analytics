package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// Item represents the data structure of an item
type Log struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Action    string    `json:"action"`
	IP        string    `josn:"ip"`
	Client    string    `json:"client"`
	Timestamp time.Time `json:"timestamp"`
}

func (app *AppSvc) storeEvent(e Event) error {
	uid := uuid.New().String()
	fmt.Println(e.UserID)
	// Create a new user in the database
	if _, err := app.DB.Exec("INSERT INTO logs(id, user_id, action, ip, client, timestamp) VALUES (?, ?, ?, ?, ?, ?)", uid, string(e.UserID), e.Action, e.IP, e.Browser, e.Timestamp); err != nil {
		log.Printf("failed to insert logs into database: %v", err)

		return err
	}

	return nil
}

// func (app *AppSvc) getItemsHandler(w http.ResponseWriter, r *http.Request) {
// 	page, err := strconv.Atoi(r.FormValue("page"))
// 	if err != nil || page < 1 {
// 		page = 1
// 	}
// 	pageSize, err := strconv.Atoi(r.FormValue("pageSize"))
// 	if err != nil || pageSize < 1 {
// 		pageSize = 10
// 	}
// 	startIndex := (page - 1) * pageSize
// 	var items []Log
// 	err = app.DB.Model(&items).Limit(pageSize).Offset(startIndex).Select()
// 	if err != nil {
// 		log.Println("Error fetching items:", err)
// 		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
// 		return
// 	}
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(items)
// }

func (app *AppSvc) getItemsHandler(w http.ResponseWriter, r *http.Request) {

	userUID, ok := r.Context().Value("userid").(string)
	if !ok {
		// Handle the case where userUID is not available in the context
		http.Error(w, "User ID not found", http.StatusUnauthorized)
		return
	}

	page, err := strconv.Atoi(r.FormValue("page"))
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(r.FormValue("pageSize"))
	if err != nil || pageSize < 1 {
		pageSize = 10
	}

	startIndex := (page - 1) * pageSize

	var items []Log

	// Use db.Exec to execute the query and fetch items
	query := fmt.Sprintf("SELECT * FROM logs where user_id = ? ORDER BY timestamp LIMIT %d OFFSET %d", pageSize, startIndex)
	_, err = app.DB.Query(&items, query, userUID)
	if err != nil {
		log.Println("Error executing query:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}
