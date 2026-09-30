package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9095"
	}

	http.HandleFunc("/webhook", handleWebhook)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("webhook-logger listening on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
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
