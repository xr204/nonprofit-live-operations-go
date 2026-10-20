package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"example.com/nonprofit-operations-board/internal/infrai"
	"example.com/nonprofit-operations-board/internal/operations"
)

const channel = "nonprofit-operations"

type server struct {
	client *gateway.Client
	now    func() time.Time
}

type receiptRequest struct {
	ReceiptID            string `json:"receipt_id"`
	AmountCents          int64  `json:"amount_cents"`
	ReceivedAt           string `json:"received_at"`
	LastVolunteerContact string `json:"last_volunteer_contact"`
	CampaignGoalCents    int64  `json:"campaign_goal_cents"`
	CampaignRaisedCents  int64  `json:"campaign_raised_cents"`
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := gateway.NewClient(key)

	s := &server{client: client, now: time.Now}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /receipts", s.receiveReceipt)
	mux.HandleFunc("POST /realtime-token", s.issueRealtimeToken)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	addr := ":8080"
	log.Printf("nonprofit dashboard publisher listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (s *server) receiveReceipt(w http.ResponseWriter, r *http.Request) {
	var in receiptRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.ReceiptID == "" || in.AmountCents <= 0 {
		http.Error(w, "valid receipt_id and amount_cents are required", http.StatusBadRequest)
		return
	}
	receivedAt, err := time.Parse(time.RFC3339, in.ReceivedAt)
	if err != nil {
		http.Error(w, "received_at must be RFC3339", http.StatusBadRequest)
		return
	}
	lastContact, err := time.Parse(time.RFC3339, in.LastVolunteerContact)
	if err != nil {
		http.Error(w, "last_volunteer_contact must be RFC3339", http.StatusBadRequest)
		return
	}

	snapshot := operations.BuildSnapshot(operations.Receipt{ReceiptID: in.ReceiptID, AmountCents: in.AmountCents, ReceivedAt: receivedAt}, lastContact, in.CampaignGoalCents, in.CampaignRaisedCents, s.now())
	metrics, err := s.client.QueryMetrics(r.Context(), "donations.received", "sum")
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	snapshot.Metrics = metrics
	if err := s.client.Publish(r.Context(), channel, snapshot, "receipt:"+in.ReceiptID); err != nil {
		writeUpstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, snapshot)
}

func (s *server) issueRealtimeToken(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		http.Error(w, "X-Client-ID is required", http.StatusBadRequest)
		return
	}
	data, err := s.client.IssueToken(r.Context(), clientID, channel)
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func writeUpstreamError(w http.ResponseWriter, err error) {
	var apiErr *gateway.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Message})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream request failed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
