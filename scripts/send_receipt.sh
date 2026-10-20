#!/bin/sh
set -eu

curl --fail-with-body \
  --request POST \
  --header 'Content-Type: application/json' \
  --data '{"receipt_id":"receipt-42","amount_cents":1000,"received_at":"2026-10-01T09:00:00Z","last_volunteer_contact":"2026-09-20T09:00:00Z","campaign_goal_cents":10000,"campaign_raised_cents":7000}' \
  http://localhost:8080/receipts
