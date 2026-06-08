# Agentic Commerce API

A Stripe-inspired, high-performance e-commerce REST API built in Go. This project demonstrates production-ready architectural patterns, including scoped access tokens, a strict payment state machine, idempotent request processing, and an asynchronous event-driven webhook delivery system.

---

## 🏗️ System Architecture

The project is structured around a clean architecture separating the routing layer, middleware filters, handlers, and memory-backed datastores:

```
                      ┌───────────────────────┐
                      │      HTTP Client      │
                      └──────────┬────────────┘
                                 │ HTTP Request (e.g. POST /v1/carts)
                                 ▼
                     ┌─────────────────────────┐
                     │    Gin Router (v1)      │
                     └──────────┬──────────────┘
                                 │
     ┌──────────────────────────┼──────────────────────────┐
     │ Middleware Pipeline      ▼                          │
     │  ├─ RequestID (Trace ID generation)                 │
     │  ├─ Idempotency (Strict double-submit prevention)  │
     │  └─ RequireScope (Scoped access tokens validation)  │
     └──────────────────────────┬──────────────────────────┘
                                 │
                                 ▼
                      ┌───────────────────────┐
                      │    Handler Layer      │
                      │  (JSON Binding & Val) │
                      └────┬──────────────┬───┘
                           │              │
        Updates Store State│              │Dispatches Async Webhook Event
                           ▼              ▼
                    ┌────────────┐  ┌──────────────┐
                    │ Data Store │  │  Dispatcher  │
                    │  (Memory)  │  └──────┬───────┘
                    └────────────┘         │ Runs background workers
                                           ▼
                                    ┌──────────────┐
                                    │  Subscribers │
                                    │ (Webhook URLs│
                                    └──────────────┘
```

---

## 🚀 Key Design Decisions

### 1. **Payment State Machine**
Payments follow a strict, deterministic state-transition graph to prevent duplicate checkouts or unauthorized order completions:
```
[created] ──► [requires_confirmation] ──► [succeeded] OR [failed]
```
*   **Terminal States:** Once a payment intent transitions to `succeeded` or `failed`, no further transitions are allowed.
*   **Type-Safe Transitions:** State changes return explicit transition errors instead of bare string mismatches, ensuring precise HTTP responses.

### 2. **Concurrency-Safe Carts (Copy-on-Read / Write)**
To avoid data races in concurrent environments, the `CartStore` implements deep copying during retrievals (`Get`) and modifications (`Save`). This decouples database memory from handler goroutines, guaranteeing thread safety without blocking the server.

### 3. **Idempotency with TTL Eviction**
`POST` endpoints accept an `Idempotency-Key` header. Requests are intercepted, processed, and their responses cached. Duplicate requests return the cached response immediately, bypassing handler execution. A background worker periodically runs to evict expired cache entries using a configurable Time-To-Live (TTL).

### 4. **Asynchronous Webhook Engine**
Webhook events are recorded in an event log and dispatched to active subscribers asynchronously. Dispatch workers process events in parallel, complete with an exponential backoff retry mechanism (max 3 retries) to handle transient client timeouts.

---

## 📂 Project Structure

```
├── config/              # Configuration loader (env vars with defaults)
├── gateway/             # External payment gateway interface & mocks
├── handlers/            # HTTP Handlers (validation and store orchestration)
├── middleware/          # Gin Middlewares (Auth, RequestID, Idempotency, Admin Key)
├── models/              # Go structs representing database entities
├── store/               # In-memory repositories & state machine
├── webhook/             # Webhook event dispatcher & backoff retry logic
├── .env.example         # Environment template file
├── .golangci.yml        # Linter configuration
├── go.mod               # Go module configuration
├── Makefile             # Build automation shortcuts
└── README.md            # Project documentation
```

---

## 🛠️ Getting Started

### Prerequisites
*   Go (version 1.22 or later)
*   GNU Make (optional, for automation commands)

### Configuration
1. Copy the environment template:
   ```bash
   cp .env.example .env
   ```
2. Modify variables inside `.env` to configure ports, timeouts, and security credentials:
   *   `PORT`: Port to run the server on (default: `8080`).
   *   `ADMIN_API_KEY`: Key used to authenticate token creation calls (`X-Admin-Key` header).
   *   `IDEMPOTENCY_TTL`: Lifetime of idempotency cache entries (default: `24h`).

### Running the Application
Using the configured Makefile:
*   **Run development server:** `make run`
*   **Build production binary:** `make build`
*   **Run linter check:** `make lint`
*   **Clean build files:** `make clean`

---

## 🔌 API Reference

### 1. Authentication
Endpoints (except for token creation) require a Bearer token in the `Authorization` header:
```http
Authorization: Bearer <your-access-secret>
```

#### **Create Token**
*   **Endpoint:** `POST /v1/tokens`
*   **Header:** `X-Admin-Key: <ADMIN_API_KEY>`
*   **Payload:**
    ```json
    {
      "scopes": ["products:read", "carts:write", "carts:read", "payments:write"],
      "spend_limit_paise": 100000,
      "expires_in": "2h"
    }
    ```
*   **Response:** `201 Created` returning the generated token containing the authenticating `secret`.

---

### 2. Purchase Flow Example

#### **Step 1: Browse Products**
*   **Request:** `GET /v1/products`
*   **Response:** `200 OK` listing the catalog of products.

#### **Step 2: Create a Cart**
*   **Request:** `POST /v1/carts`
*   **Response:** `201 Created` with a new cart object:
    ```json
    {
      "id": "cart_uuid_goes_here",
      "items": [],
      "total_paise": 0,
      "created_at": "...",
      "updated_at": "..."
    }
    ```

#### **Step 3: Add Items to Cart**
*   **Request:** `POST /v1/carts/<cart_id>/items`
*   **Payload:**
    ```json
    {
      "product_id": "prod_1",
      "quantity": 2
    }
    ```
*   **Response:** `200 OK` showing the updated cart state and recalculated price total.

#### **Step 4: Create a Payment Intent**
*   **Request:** `POST /v1/payment-intents`
*   **Payload:**
    ```json
    {
      "cart_id": "cart_uuid_goes_here",
      "currency": "INR"
    }
    ```
*   **Response:** `201 Created` with a payment intent in the `requires_confirmation` status.

#### **Step 5: Confirm Payment**
*   **Request:** `POST /v1/payment-intents/<intent_id>/confirm`
*   **Response:** `200 OK` with the intent status updated to `succeeded`.
