# Delivery Management System

[![Go](https://img.shields.io/badge/go-1.21%2B-blue)](https://golang.org) [![License: MIT](https://img.shields.io/badge/license-MIT-green)](./LICENSE) [![Build](https://img.shields.io/badge/build-passing-brightgreen)](#) [![Coverage](https://img.shields.io/badge/coverage-—%25-lightgrey)](#)

**Author:** Yogesh Gadhewal

---

## 📖 Project Overview

A production-ready **Delivery Management System** implemented in **Go (1.21+)**, built for scalability, observability and maintainability. The service provides secure order lifecycle management, role-based access control, real-time order tracking (WebSocket + Redis Pub/Sub), background processing, caching, and an ergonomic admin surface.

This repository is structured for use as a standalone service or as part of a microservices ecosystem.

---

## 📚 Table of Contents

* [Key Features](#key-features)
* [Tech Stack](#tech-stack)
* [Architecture](#architecture)
* [Getting Started](#getting-started)

  * [Prerequisites](#prerequisites)
  * [Clone & Install](#clone--install)
  * [Environment Variables (`.env.example`)](#environment-variables-envexample)
  * [Run](#run)
* [API Reference (important routes)](#api-reference-important-routes)
* [WebSocket / Realtime](#websocket--realtime)
* [Order Status Flow](#order-status-flow)
* [Testing](#testing)
* [Caching & Background Workers](#caching--background-workers)
* [Development Guidelines](#development-guidelines)
* [Observability & Monitoring](#observability--monitoring)
* [Migrations & DB](#migrations--db)
* [Contributing](#contributing)
* [License](#license)
* [Support & Contact](#support--contact)

---

## ✨ Key Features

* JWT authentication with role-based access control (Customer / Admin)
* Secure password hashing (bcrypt)
* Full CRUD for orders + validated order-state state machine
* WebSocket realtime updates (Gorilla WebSocket)
* Redis for caching & Pub/Sub across instances
* Background workers for status progression, notifications and retries
* GORM-based DB layer with optional auto-migrations
* Swagger/OpenAPI documentation
* Unit, integration and concurrency tests

---

## 🧰 Tech Stack

* **Language:** Go (1.21+)
* **HTTP & Routing:** Gin
* **ORM:** GORM
* **Database:** PostgreSQL (12+, recommended 15)
* **Cache / PubSub:** Redis
* **Realtime:** Gorilla WebSocket
* **Docs & Tests:** Swagger / OpenAPI, Testify

---

## 🏗 Architecture

```
Client (Web/Mobile)
   └── HTTP / WebSocket
        └── API (Gin + services)
             ├── PostgreSQL (persistent storage)
             └── Redis (cache + Pub/Sub)
```

High-level folder structure

```
delivery-management-system/
├── cmd/
│   └── server/
│       └── main.go          # Application entry point
├── internal/
│   ├── config/              # Config parsing & validation
│   ├── models/              # DB models
│   ├── handlers/            # HTTP handlers
│   ├── middleware/          # Auth, logging, rate-limit
│   ├── services/            # Business logic & workers
│   ├── auth/                # JWT & password utilities
│   └── utils/               # Helpers
├── tests/                   # Unit/integration/concurrency tests
├── docs/                    # Swagger / OpenAPI definition
└── scripts/                 # Migration & maintenance scripts
```

---

## ⚙️ Getting Started

### Prerequisites

* Go 1.21+
* PostgreSQL 12+ (Postgres 15 recommended)
* Redis 6+

### Clone & Install

```bash
git clone https://github.com/your-username/delivery-management-system.git
cd delivery-management-system
go mod tidy
```

### Environment Variables (`.env.example`)

Create a `.env` file from `.env.example` (do **not** commit secrets).

```env
ENVIRONMENT=development
PORT=8080

DATABASE_URL=postgres://postgres:password@localhost:5432/delivery_db?sslmode=disable
REDIS_URL=redis://localhost:6379
JWT_SECRET=super-secret-jwt-key

# Optional tuning
BCRYPT_COST=12
GORM_MAX_IDLE_CONNS=10
GORM_MAX_OPEN_CONNS=50
WORKER_POLL_INTERVAL=5s
```

> **Important:** Use a secrets manager for production secrets. Never commit real secrets to the repository.

### Run (development)

Auto-migrations will run on startup if enabled in configuration.

```bash
# Run directly
go run cmd/server/main.go

# Or build and run
go build -o bin/delivery ./cmd/server
./bin/delivery
```

Health check

```bash
curl http://localhost:8080/health
```

---

## 🔎 API Reference (Important Routes)

> Base URL: `http://localhost:8080`

All protected endpoints require: `Authorization: Bearer <JWT>`

### Auth

* `POST /auth/register` — Register a new user
* `POST /auth/login` — Login (returns JWT)

### Profile

* `GET /api/profile` — Get current user profile

### Orders (Customer)

* `POST /api/orders` — Create a new order
* `GET /api/orders` — List user's orders (supports pagination & filters)
* `GET /api/orders/{id}` — Get order by ID
* `PATCH /api/orders/{id}/cancel` — Cancel an order (allowed before delivered)
* `GET /api/orders/status-flow` — Returns allowed status transitions

### Admin

* `PATCH /api/orders/{id}/status` — Update order status (admin only)
* `GET /api/admin/active-orders` — Active orders count & list
* `GET /api/admin/websocket-clients` — Connected WebSocket clients
* `POST /api/admin/broadcast-message` — Broadcast system message
* `GET /api/admin/realtime-stats` — Realtime metrics

### WebSocket (Realtime)

* `GET /ws?token=<JWT>` — Connect to WebSocket endpoint for live events

Sample WebSocket event: `order_status_changed`

```json
{
  "type": "order_status_changed",
  "data": {
    "order_id": "uuid-123",
    "status": "in_transit",
    "timestamp": "2025-10-12T10:05:00Z"
  }
}
```

---

## 🔄 Order Status Flow

```
Created → Dispatched → In Transit → Delivered
    ↓
 Cancelled (allowed any time before Delivered)
```

**Valid transitions (service-layer enforced):**

* `created -> dispatched | cancelled`
* `dispatched -> in_transit | cancelled`
* `in_transit -> delivered | cancelled`
* `delivered` and `cancelled` are terminal states

---

## 🧪 Testing

Run the entire test suite locally:

```bash
go test ./tests/... -v
```

Run specific suites:

```bash
# Unit
go test ./tests/unit/... -v

# Integration
go test ./tests/integration/... -v

# Concurrency
go test ./tests/concurrency/... -v

# With coverage
go test ./... -cover
```

**Recommended coverage targets:** Unit 90%+, Integration 80%+

---

## 🗄 Caching & Background Workers

* **Redis caching:** used for frequently-read resources (e.g., active order counters). Cache invalidation occurs on writes/updates.
* **Redis Pub/Sub:** used to propagate events across instances for the WebSocket layer.
* **Background workers:** handle status progression, notification delivery, and processing of long-running jobs. Workers should be idempotent and use retry/backoff strategies.

---

## 🛠 Development Guidelines

* Keep HTTP handlers thin; push business logic into `services/`.
* Use dependency injection for DB/Redis/Worker clients (interfaces make testing easier).
* Validate inputs using Gin binding + custom validators.
* Use `context` with timeouts for DB and network calls.
* Keep errors in a consistent JSON structure: `{ code, message, details }`.
* Do not put business logic in Redis; use Redis as cache/pub-sub only.

---

## 📈 Observability & Monitoring

* Structured logging (JSON) with levels (INFO/WARN/ERROR).
* Expose `/metrics` for Prometheus scraping when configured.
* Health checks: `/health` (liveness) and `/ready` (readiness).
* Integrate tracing (OpenTelemetry recommended) in production.

---

## ✅ Migrations & DB

* Use a dedicated migrations tool (recommended: `golang-migrate`) or enable controlled GORM auto-migrations behind a config flag.
* Store migration files under `scripts/migrations/` or `internal/db/migrations/`.

---

## 🤝 Contributing

1. Fork the repository.
2. Create a branch: `git checkout -b feature/awesome`.
3. Implement changes and add tests.
4. Run the test suite and ensure linting passes.
5. Open a PR with a clear description and link to related issue.

**PR checklist:**

* Tests added/updated
* Documentation updated (README / Swagger)
* `go vet` and linter clean

---

## 📜 License

This project is licensed under the **MIT License**. See `LICENSE` for full text.

```text
MIT License

Copyright (c) 2025 Yogesh Gadhewal

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

```

---

## 🆘 Support & Contact

* GitHub Issues: `https://github.com/your-username/delivery-management-system/issues`
* For private/urgent support: add your preferred contact (email, Slack, Discord).

---

