# DevAI Infra Prototype 🚀

**B2B Open-Source Platform for AI-Agents Governance & Execution Control.**

Этот проект реализует концепцию единого контрольного слоя для управления AI-агентами в корпоративной среде. Мы решаем проблему «неуправляемого доступа» агентов к API, превращая хаос интеграций в прозрачную и безопасную операционную модель.

## 🏗 Ключевая архитектура: Gateway → Policy → Execution → Audit
Агенты никогда не получают прямой доступ к API. Каждый шаг проходит через **UAG (Unified Agent Gateway)**.

### Технологический стек:
- **Language:** Go 1.21+ (Zero-allocation path focus)
- **Control Plane:** Redis (Pub/Sub + Distributed Sets)
- **Storage:** PostgreSQL 15 (pgx driver)
- **API/RPC:** gRPC (Protobuf), REST (chi)
- **Resilience:** Circuit Breaker, Exponential Backoff, Rate Limiting

---

## 🛠 Ключевые возможности

*   **Kill-Switch (Real-time):** Мгновенная блокировка агента во всем кластере шлюзов через Redis-сигналы.
*   **Sandbox Mode:** Режим «песочницы» - выполнение перехватывается, логируется в аудит, но не затрагивает реальные данные (Dry Run).
*   **AgentFS Audit Trail:** Асинхронная пакетная запись (Batching) всех действий агентов в БД без задержек для Hot Path.
*   **Reliability Layer:** Встроенные механизмы защиты от каскадных сбоев (Circuit Breaker) и умные ретраи (v5).

## 📂 Структура проекта

Проект организован в соответствии со стандартным Go-лейаутом. Основная логика разделена на независимые слои (Transport, Service, Repository).

```text
devit-core/
├── api/                  # Исходные Protobuf-контракты (gRPC/REST)
├── build/                # Dockerfiles для Console и UAG Engine
├── cmd/                  
│   ├── console/main.go   # Точка входа в консоль
│   └── uag/main.go       # Точка входа в шлюз
├── docs/                 # Архитектурные описания и диаграммы
├── internal/             # Приватная бизнес-логика (не импортируется извне)
│   ├── audit/            # Асинхронный аудит (AgentFS) и сбор/запись событий
│   ├── console/          # Обработка запросов админ-панели
│   ├── engine/           # Ядро шлюза (UAG Core, Kill-switch, Sandbox)(Flow:Policy -> Exec)
│   ├── policy/           # Слой политик (PDP/PEP интерфейсы)
│   └── connectors/       # Адаптеры к внешним системам (Jira, DB) реализация SDK(gRPC/REST)
├── migrations/           # SQL-миграции (PostgreSQL schema)
├── pkg/                  # Публичные библиотеки и сгенерированный код API
│   └── api/              # Protobuf / OpenAPI контракты
├── scripts/              # SQL-запросы для отладки и демо-сценарии
├── docker-compose.yaml
└── Makefile              # Автоматизация: gen-proto, build, test
```
---

## 🚀 Быстрый запуск (Docker Compose)

Система готова к работе «из коробки» (включая БД, Redis и автоматические миграции):

```bash
docker-compose up -d --build
```
#### Проверка работы (End-to-End):
Инструкции и сценарии тестирования находятся в папке scripts/.

## 📖 Документация и Deep Dive
Подробный разбор архитектурных решений, структуры БД и логики работы слоев находится здесь:
* 👉 [Архитектурная документация (docs/README.md)](./docs/README.md)

## Deployment Options

This Gateway is designed to be flexible and scale with your needs.

### 1. Community Edition (Open Source)
The core engine is open-source and free to use. Ideal for developers, local testing, and small-scale integrations.
*   **Self-managed:** You handle infrastructure, scaling, and security updates.
*   **License:** MIT / Apache 2.0 (поставь нужную).

### 2. Managed Gateway (Cloud) — Coming Soon!
For enterprise-grade reliability and zero-config deployment.
*   **High Availability:** Managed infrastructure with 99.9% uptime.
*   **Security & Compliance:** Built-in auditing, PII masking, and RBAC.
*   **Advanced Analytics:** Detailed dashboards for AI agent performance and cost tracking.
*   **Global Latency Optimization:** Multi-region deployments.

> [!TIP]
> Need custom integration or AI architecture consulting? [Contact our team](mailto:xela07ax@gmail.com).


## ⚖️ Лицензия
Distributed under the Apache 2.0 License.
