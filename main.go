package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

var bearerToken string

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9095"
	}

	// Token can be supplied as literal or read from a file path.
	bearerToken = os.Getenv("WEBHOOK_TOKEN")
	if tokenFile := os.Getenv("WEBHOOK_TOKEN_FILE"); tokenFile != "" {
		data, err := os.ReadFile(tokenFile)
		if err != nil {
			log.Fatalf("failed to read token file %s: %v", tokenFile, err)
		}
		bearerToken = strings.TrimSpace(string(data))
	}

	http.HandleFunc("/webhook", handleWebhook)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("webhook-logger listening on :%s (auth=%v)", port, bearerToken != "")
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check Bearer token if configured
	if bearerToken != "" {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != bearerToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Decode the webhook envelope while preserving all individual alert fields.
	var payload struct {
		Alerts []map[string]json.RawMessage `json:"alerts"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if payload.Alerts == nil {
		http.Error(w, "missing alerts array", http.StatusBadRequest)
		return
	}
	for _, alert := range payload.Alerts {
		if alert == nil {
			http.Error(w, "invalid alert object", http.StatusBadRequest)
			return
		}
	}

	// Emit one compact JSON line per alert, using its individual status.
	encoder := json.NewEncoder(os.Stdout)
	for _, alert := range payload.Alerts {
		if err := encoder.Encode(alert); err != nil {
			log.Printf("failed to write alert: %v", err)
			http.Error(w, "failed to write alert", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}
