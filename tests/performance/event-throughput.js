import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

const baseUrl = (__ENV.BASE_URL || "http://localhost:8088").replace(/\/$/, "");
const apiKey = __ENV.API_KEY || "local-dev-api-key-change-me";
const timeoutMs = Number(__ENV.EVENT_TIMEOUT_MS || 10000);
const eventDelivery = new Trend("finflow_event_delivery_duration", true);
const eventFailures = new Rate("finflow_event_delivery_failed");

export const options = {
  scenarios: {
    event_throughput: {
      executor: "constant-vus",
      vus: Number(__ENV.VUS || 5),
      duration: __ENV.DURATION || "1m",
      gracefulStop: "15s",
    },
  },
  thresholds: {
    checks: ["rate>0.99"],
    http_req_failed: ["rate<0.01"],
    finflow_event_delivery_failed: ["rate<0.01"],
    finflow_event_delivery_duration: ["p(95)<5000", "p(99)<10000"],
  },
};

export default function () {
  const unique = `${__VU}-${__ITER}-${Date.now()}`;
  const create = http.post(
    `${baseUrl}/api/v1/payments`,
    JSON.stringify({
      amount_cents: 1299,
      currency: "USD",
      description: "FinFlow event capacity test",
      external_reference: `event-load-${unique}`,
    }),
    {
      headers: {
        "Content-Type": "application/json",
        "X-API-Key": apiKey,
        "Idempotency-Key": `event-load-${unique}`,
      },
      tags: { endpoint: "event_create_payment" },
    },
  );
  if (!check(create, { "event payment created": (response) => response.status === 201 })) {
    eventFailures.add(true);
    return;
  }

  const paymentId = create.json("id");
  const started = Date.now();
  let delivered = false;
  while (Date.now() - started < timeoutMs) {
    const entries = http.get(`${baseUrl}/api/v1/ledger/payments/${paymentId}/entries`, {
      headers: { "X-API-Key": apiKey },
      tags: { endpoint: "event_ledger_poll" },
    });
    if (entries.status === 200 && entries.json("entries.#") === 2) {
      delivered = true;
      break;
    }
    sleep(0.25);
  }

  eventDelivery.add(Date.now() - started);
  eventFailures.add(!delivered);
  check(delivered, { "payment event reached balanced ledger": (value) => value });
}
