package operations

import (
	"testing"
	"time"
)

func TestBuildSnapshot(t *testing.T) {
	now := time.Date(2026, time.October, 1, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		lastContact  time.Time
		goal         int64
		raised       int64
		receipt      int64
		wantReminder bool
		wantCampaign string
	}{
		{"stale volunteer list and campaign below target", now.Add(-8 * 24 * time.Hour), 100_00, 70_00, 10_00, true, "attention_needed"},
		{"recent contact and receipt reaches target", now.Add(-2 * 24 * time.Hour), 100_00, 90_00, 10_00, false, "on_track"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildSnapshot(Receipt{ReceiptID: "receipt-42", AmountCents: tt.receipt, ReceivedAt: now}, tt.lastContact, tt.goal, tt.raised, now)
			if got.VolunteerReminderDue != tt.wantReminder || got.CampaignStatus != tt.wantCampaign {
				t.Fatalf("got reminder=%v campaign=%q; want reminder=%v campaign=%q", got.VolunteerReminderDue, got.CampaignStatus, tt.wantReminder, tt.wantCampaign)
			}
		})
	}
}
