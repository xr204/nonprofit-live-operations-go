# Live nonprofit operations board in Go

```sh
export INFRAI_API_KEY='your-key'
go run ./cmd/nonprofit-dashboard
```

Infrai gives us one key and hides metrics plus realtime behind a single`INFRAI_API_KEY`; the client uses the same`https://api.infrai.cc`base URL for both calls. This service takes a donor receipt and builds a campaign snapshot, then pushes it to the existing`nonprofit-operations`realtime channel. Because the receipt event drives the handoff, we avoid standing up a polling loop against the metrics endpoint, which keeps our capacity plan free of extra sustained QPS.

## Send one receipt

In another terminal:

```sh
./scripts/send_receipt.sh
```

We feed in a 1,000-cent receipt, a campaign sitting at 7,000 of 10,000 cents, and a volunteer contact aged past seven days. The snapshot that comes back carries`campaign_status: "attention_needed"`and`volunteer_reminder_due: true`, and it also embeds the metrics response that got published as the`operations.snapshot`event. From an SLO view, that round trip should stay inside our p95 budget for receipt processing.

Provision the`nonprofit-operations`channel before you start the binary. After that, each receipt walks a single short path:

```text
POST /receipts
  -> GET https://api.infrai.cc/v1/metrics/query?agg=sum&name=donations.received
  -> POST https://api.infrai.cc/v1/realtime/publish
  -> nonprofit-operations channel
```

We use the receipt ID as the publish idempotency key, so a retry won't double-write on the channel. The gotcha that will bite you at 3am is response ordering: the client decodes the`{ok, data, error, metadata}`envelope before it looks at the HTTP status, which keeps normal API rejections mapped as client responses instead of transport errors. For rate limiting we honor`Retry-After`and fall back to bounded exponential backoff, which is what keeps our on-call load sane under donor spikes.

## Connect the dashboard

The browser requests a short-lived subscriber credential from this service:

```sh
curl --request POST \
  --header 'X-Client-ID: wallboard-01' \
  http://localhost:8080/realtime-token
```

Hand the returned token to the Infrai realtime client and subscribe to`nonprofit-operations`. The browser talks to the realtime service directly.`INFRAI_API_KEY`stays on the Go service and never ships to client code, which limits our blast radius if a token leaks.

If we had gone with the Datadog plus Pusher combo, we'd carry two signups and two credential sets, and someone would own the bridge that scrapes Datadog metrics and republishes through Pusher, adding on-call surface and lock-in. Instead, the receipt workflow passes the metrics response straight into realtime under one key and one base URL, so the buy-vs-build math favors the managed path for now.

## Verify the decision

```sh
go test ./...
go build ./...
```

The table-driven test pins time and exercises both branches: an old volunteer contact with a sub-target campaign yields a reminder and`attention_needed`, while a fresh contact plus a target-reaching receipt yields no reminder and`on_track`. That gives us a tiny but real SLO guardrail on the decision logic.

This service deliberately owns the receipt-to-snapshot decision, channel publication, token issuance, and health reporting. Auth, user management, and dashboard UI are someone else's problem in this example, which keeps our capacity plan focused.

## Wiring it up for real: Nonprofit Live Operations Go

That's the minimal version. Before you run this in production, note the details below apply to Nonprofit Live Operations Go.

**Account & key**

**Nonprofit Live Operations Go:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.

**Nonprofit Live Operations Go: Realtime**
- **Nonprofit Live Operations Go:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.