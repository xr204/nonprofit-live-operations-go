package operations

import "time"

// Receipt is the input that triggers a fresh operational snapshot.
type Receipt struct {
	ReceiptID   string    `json:"receipt_id"`
	AmountCents int64     `json:"amount_cents"`
	ReceivedAt  time.Time `json:"received_at"`
}

// Snapshot records the business decision shown on the dashboard.
type Snapshot struct {
	ReceiptID            string `json:"receipt_id"`
	AmountCents          int64  `json:"amount_cents"`
	VolunteerReminderDue bool   `json:"volunteer_reminder_due"`
	CampaignStatus       string `json:"campaign_status"`
	Metrics              any    `json:"metrics"`
}

func BuildSnapshot(receipt Receipt, lastVolunteerContact time.Time, campaignGoalCents, campaignRaisedCents int64, now time.Time) Snapshot {
	status := "on_track"
	if campaignRaisedCents+receipt.AmountCents < campaignGoalCents {
		status = "attention_needed"
	}

	return Snapshot{
		ReceiptID:            receipt.ReceiptID,
		AmountCents:          receipt.AmountCents,
		VolunteerReminderDue: now.Sub(lastVolunteerContact) >= 7*24*time.Hour,
		CampaignStatus:       status,
	}
}
