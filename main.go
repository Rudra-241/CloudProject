package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

type Message struct {
	ID      int    `json:"id"`
	Topic   string `json:"topic"`
	Content string `json:"content"`
}

var (
	subscribers = make(map[string][]chan Message)
	mu          sync.Mutex
	db          *sql.DB
)

func main() {
	var err error
	db, err = sql.Open("sqlite3", "./messages.db")
	if err != nil {
		log.Fatal(err)
	}
	initDB()

	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/publish", publishHandler)
	http.HandleFunc("/subscribe", subscribeHandler)
	http.ListenAndServe(":8080", nil)
}

func initDB() {
	query := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		topic TEXT,
		content TEXT
	)`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatal("DB Init:", err)
	}
}

func publishHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var msg Message
	err := json.NewDecoder(r.Body).Decode(&msg)
	if err != nil {
		http.Error(w, "Invalid", http.StatusBadRequest)
		return
	}

	res, err := db.Exec("INSERT INTO messages(topic, content) VALUES(?, ?)", msg.Topic, msg.Content)
	if err != nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()
	msg.ID = int(id)

	mu.Lock()
	for _, ch := range subscribers[msg.Topic] {
		go func(c chan Message) { c <- msg }(ch)
	}
	mu.Unlock()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(msg)
}

func subscribeHandler(w http.ResponseWriter, r *http.Request) {
	topic := r.URL.Query().Get("topic")
	if topic == "" {
		http.Error(w, "Missing topic", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	msgChan := make(chan Message)

	mu.Lock()
	subscribers[topic] = append(subscribers[topic], msgChan)
	mu.Unlock()

	rows, err := db.Query("SELECT id, topic, content FROM messages WHERE topic = ? ORDER BY id ASC", topic)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var msg Message
			rows.Scan(&msg.ID, &msg.Topic, &msg.Content)
			fmt.Fprintf(w, "data: %s\n\n", jsonMessage(msg))
			flusher.Flush()
		}
	}

	for {
		msg := <-msgChan
		fmt.Fprintf(w, "data: %s\n\n", jsonMessage(msg))
		flusher.Flush()
	}
}

func jsonMessage(msg Message) string {
	jsonMsg, _ := json.Marshal(msg)
	return string(jsonMsg)
}
