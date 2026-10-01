import http from "k6/http";
import { check } from "k6";

const baseUrl = (__ENV.BASE_URL || "http://localhost:8088").replace(/\/$/, "");
const apiKey = __ENV.API_KEY || "local-dev-api-key-change-me";

export const options = {
  scenarios: {
    payment_api: {
      executor: "constant-vus",
      vus: Number(__ENV.VUS || 10),
      duration: __ENV.DURATION || "1m",
      gracefulStop: "10s",
    },
  },
  thresholds: {
    checks: ["rate>0.99"],
    http_req_failed: ["rate<0.01"],
    "http_req_duration{endpoint:create_payment}": ["p(95)<300", "p(99)<750"],
    "http_req_duration{endpoint:get_payment}": ["p(95)<150", "p(99)<500"],
  },
};

export default function () {
  const unique = `${__VU}-${__ITER}-${Date.now()}`;
  const headers = {
    "Content-Type": "application/json",
    "X-API-Key": apiKey,
    "Idempotency-Key": `load-${unique}`,
  };
  const create = http.post(
    `${baseUrl}/api/v1/payments`,
    JSON.stringify({
      amount_cents: 1299,
      currency: "USD",
      description: "FinFlow capacity test",
      external_reference: `load-${unique}`,
    }),
    { headers, tags: { endpoint: "create_payment" } },
  );
  const created = check(create, {
    "payment created": (response) => response.status === 201,
    "payment id returned": (response) => Boolean(response.json("id")),
  });
  if (!created) {
    return;
  }

  const paymentId = create.json("id");
  const get = http.get(`${baseUrl}/api/v1/payments/${paymentId}`, {
    headers: { "X-API-Key": apiKey },
    tags: { endpoint: "get_payment" },
  });
  check(get, {
    "payment read": (response) => response.status === 200,
    "payment identity preserved": (response) => response.json("id") === paymentId,
  });
}
