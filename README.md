# PulsoATS — Main Service

HTTP API-сервис платформы PulsoATS. Управляет пользователями, биржевыми аккаунтами, нодами воркеров и прогонами торговых стратегий в реальном времени.

## Содержание

- [Архитектура](#архитектура)
- [Домены](#домены)
- [Быстрый старт](#быстрый-старт)
- [Конфигурация](#конфигурация)
- [Миграции](#миграции)
- [API](#api)
- [Деплой нод и воркеров](#деплой-нод-и-воркеров)
- [Разработка](#разработка)

---

## Архитектура

```
cmd/
└── main.go                        # Точка входа, wire-up зависимостей

internal/
├── application/                   # Бизнес-логика (use cases)
│   ├── analysis/                  # Бэктест-прогоны
│   ├── auth/                      # Аутентификация и сессии
│   ├── live/                      # Биржевые аккаунты, ноды, воркеры
│   └── marketSpec/                    # Справочник символов
├── domain/                        # Доменные модели и интерфейсы репозиториев
│   ├── live/                      # ExchangeAccount, Node, Worker, Event, Metrics
│   └── ...
├── infrastructure/
│   ├── certgen/                   # Генерация TLS-сертификатов для воркеров (CA-подпись)
│   ├── cryptox/                   # AES-GCM шифрование учётных данных бирж
│   ├── docker/                    # Docker-клиент: деплой контейнеров воркеров и БД на нодах
│   ├── grpc/
│   │   ├── analysis/              # gRPC-клиент сервиса анализа
│   │   └── live/                  # gRPC-клиент воркера (health, runs, metrics, events)
│   ├── email/aws-ses/             # Отправка писем через Amazon SES v2
│   └── repository/postgres/       # PostgreSQL-репозитории (pgx v5)
└── transport/
    ├── handlers/                  # HTTP-обработчики (Gin)
    ├── middleware/                 # Auth, CORS, логирование
    └── router/                    # Сборка маршрутов
```

**Стек:** Go 1.26.2 · Gin · pgx v5 · TimescaleDB · Docker API · gRPC · AWS SES

---

## Домены

### Auth
Регистрация по инвайт-токенам, JWT (access + refresh), подтверждение email, сброс пароля, управление сессиями. Пароли — Argon2id.

### Live
Основной домен платформы. Три сущности:

| Сущность | Описание |
|----------|----------|
| **ExchangeAccount** | Биржевой аккаунт пользователя с зашифрованными API-ключами (AES-256-GCM). Имеет срок действия. |
| **Node** | Удалённый Docker-хост. При регистрации разворачивает TimescaleDB (или использует готовую БД по DSN). |
| **Worker** | Контейнер с live-воркером на ноде. Один аккаунт — один воркер. Управляется асинхронно. |

**Жизненный цикл воркера:**
```
[нет] → POST /worker → deploying → running
                                  ↓
              POST /worker/start ← stopped
                                  ↓
              POST /worker/stop → stopped
```

**SSE-потоки:**
- `GET /accounts/:id/events` — события воркера (прогоны, сигналы)
- `GET /accounts/:id/worker/metrics` — метрики контейнера (CPU, RAM)
- `GET /accounts/:id/worker/stats` — статистика прогонов

### Analysis
Бэктест торговых стратегий на исторических данных. Прогоны запускаются через gRPC-сервис анализа, результаты стримятся по SSE.

### Market
Справочник торговых инструментов с автодополнением.

---

## Быстрый старт

### Зависимости

- Docker и Docker Compose
- Go 1.26.2+
- Доступ к приватным модулям `github.com/pulsoats/*` (токен GitHub)

### Запуск

```bash
# 1. Скопировать конфиг
cp .env.example .env
# Заполнить обязательные переменные (см. раздел Конфигурация)

# 2. Запустить PostgreSQL (TimescaleDB)
docker compose up postgres -d

# 3. Применить миграции
docker compose run --rm migrate

# 4. Запустить сервис
go run ./cmd
```

### Docker

```bash
# Сборка образа (требует GITHUB_TOKEN для приватных модулей)
DOCKER_BUILDKIT=1 docker build \
  --secret id=github_token,env=GITHUB_TOKEN \
  -t pulsoats-main .

# Полный стек
docker compose up -d
```

---

## Конфигурация

Все переменные окружения — в `.env.example`. Ниже — обязательные.

### HTTP

| Переменная | По умолчанию | Описание |
|------------|-------------|----------|
| `HTTP_ADDR` | `:8080` | Адрес HTTP-сервера |
| `APP_FRONTEND_URL` | — | URL фронтенда (CORS, ссылки в письмах) |
| `CORS_ALLOWED_ORIGINS` | = `APP_FRONTEND_URL` | Разрешённые origins через запятую |
| `APP_NAME` | — | Название приложения (письма) |
| `JWT_SECRET` | — | Секрет для подписи JWT |

### PostgreSQL

| Переменная | Описание |
|------------|----------|
| `POSTGRES_DSN` | DSN подключения к TimescaleDB |
| `POSTGRES_USER` | Пользователь (для docker-compose healthcheck) |
| `POSTGRES_PASSWORD` | Пароль |
| `POSTGRES_DB` | Имя базы данных |

### gRPC mTLS

Сертификаты монтируются в контейнер из директории `CERTS_DIR` (по умолчанию `./certs`).

| Переменная | Описание |
|------------|----------|
| `GRPC_TLS_DISABLE` | `true` — отключить TLS (только для разработки) |
| `GRPC_CERT_FILE` | Путь к клиентскому сертификату |
| `GRPC_KEY_FILE` | Путь к приватному ключу клиента |
| `GRPC_CA_FILE` | Путь к корневому CA-сертификату |
| `GRPC_CA_KEY_FILE` | Путь к приватному ключу CA (для подписи сертов воркеров) |
| `CERTS_DIR` | Директория с сертами на хосте, монтируется в `/run/certs` (только docker compose, по умолчанию `./certs`) |

### Шифрование учётных данных

```bash
# Генерация ключа
openssl rand -hex 32
```

| Переменная | Описание |
|------------|----------|
| `CREDENTIALS_KEY` | 32-байтный AES-ключ в hex (64 символа) |

### Docker (ноды)

Сертификаты для подключения к удалённым Docker-демонам на нодах.

| Переменная | Описание |
|------------|----------|
| `DOCKER_CA_CERT_FILE` | Путь к CA-сертификату Docker |
| `DOCKER_CERT_FILE` | Путь к клиентскому сертификату Docker |
| `DOCKER_KEY_FILE` | Путь к ключу клиента Docker |

### Реестр образов (GHCR)

| Переменная | Описание |
|------------|----------|
| `GHCR_USER` | Логин GitHub Container Registry |
| `GHCR_TOKEN` | PAT с правом `read:packages` |
| `LIVE_IMAGE_URL` | Образ live-воркера, например `ghcr.io/pulsoats/live:latest` |
| `DOCKER_DB_IMAGE` | Образ БД для нод (по умолчанию `timescale/timescaledb:latest-pg17`) |

### AWS SES

| Переменная | Описание |
|------------|----------|
| `SES_ACCESS_KEY` / `SES_SECRET_KEY` | AWS-ключи |
| `SES_REGION` | Регион, например `eu-central-1` |
| `SES_SENDER` | Адрес отправителя |
| `SES_BASE_ENDPOINT` | Опционально — кастомный endpoint (для тестов) |

### Bootstrap-администратор

Если заданы оба, при старте создаётся root-пользователь с ролью `admin`.

| Переменная | Описание |
|------------|----------|
| `ROOT_ADMIN_EMAIL` | Email администратора |
| `ROOT_ADMIN_PASSWORD` | Пароль (мин. 8 символов) |

---

## Миграции

Миграции применяются через [golang-migrate](https://github.com/golang-migrate/migrate).

```bash
# Через docker compose
docker compose run --rm migrate

# Вручную
migrate -path ./migrations -database "$POSTGRES_DSN" up

# Откат
migrate -path ./migrations -database "$POSTGRES_DSN" down 1
```

**Таблицы:**

| Миграция | Таблицы |
|----------|---------|
| `001_create_auth` | `auth.users`, `auth.sessions`, `auth.invite_tokens`, `auth.password_reset_tokens` |
| `002_create_live_tables` | `exchange_accounts`, `nodes`, `workers` |
| `003_create_markets` | `markets` |

---

## API

Полная спецификация — [`docs/openapi.yaml`](docs/openapi.yaml).

**Базовый URL:** `https://api.pulsoats.com`

**Аутентификация:** Bearer JWT (`Authorization: Bearer <access_token>`)

### Основные группы маршрутов

| Группа | Маршруты | Описание |
|--------|----------|----------|
| `POST /auth/register` | публичный | Регистрация по инвайт-токену |
| `POST /auth/login` | публичный | Вход, возвращает access + refresh токены |
| `GET /accounts` | авторизован | Список биржевых аккаунтов |
| `POST /accounts` | авторизован | Создать аккаунт |
| `GET /accounts/:id/worker` | авторизован | Воркер аккаунта |
| `POST /accounts/:id/worker` | авторизован | Создать и задеплоить воркер (202) |
| `POST /accounts/:id/worker/start` | авторизован | Запустить остановленный воркер (202) |
| `POST /accounts/:id/worker/stop` | авторизован | Остановить воркер |
| `GET /accounts/:id/events` | авторизован | SSE-поток событий |
| `GET /accounts/:id/worker/metrics` | авторизован | SSE-метрики контейнера |
| `GET /accounts/:id/runs` | авторизован | Прогоны аккаунта |
| `POST /accounts/:id/runs` | авторизован | Запустить прогон |
| `GET /workers` | авторизован | Все воркеры (фильтр: `?exchange=`, `?node_id=`) |
| `POST /admin/nodes` | admin | Добавить ноду (202) |
| `GET /admin/nodes` | admin | Список нод (фильтр: `?exchange=`) |
| `POST /admin/nodes/:id/disable` | admin | Отключить ноду (202) |
| `POST /admin/nodes/:id/enable` | admin | Включить ноду |

---

## Деплой нод и воркеров

### Добавление ноды

Нода — это удалённый Docker-хост, на котором разворачиваются воркеры.

**Режим 1 — новая БД** (TimescaleDB разворачивается автоматически):
```json
POST /admin/nodes
{
  "name": "eu-central-1-node-01",
  "exchange": "bybit",
  "host": "10.0.0.5",
  "dockerPort": 2376,
  "region": "eu-central-1",
  "maxWorkers": 10,
  "dbUser": "live",
  "dbPassword": "secret"
}
```

**Режим 2 — существующая БД** (TimescaleDB уже запущена на хосте):
```json
POST /admin/nodes
{
  "name": "eu-central-1-node-01",
  "exchange": "bybit",
  "host": "10.0.0.5",
  "dockerPort": 2376,
  "region": "eu-central-1",
  "maxWorkers": 10,
  "dsn": "postgres://live:secret@localhost:5432/live?sslmode=disable"
}
```

Ответ — `202 Accepted`. Статус ноды отслеживать через `GET /admin/nodes/:node_id`:
- `deploying` → `active` (успешно)
- `deploying` → `failed` (ошибка в `lastError`)

### Создание воркера

Воркер автоматически назначается на наименее загруженную ноду для нужной биржи:

```json
POST /accounts/:account_id/worker
```

Ответ — `202 Accepted`. Статус: `deploying` → `running` / `failed`.

### Восстановление после перезапуска

При старте сервис автоматически восстанавливает Docker и gRPC-клиенты из БД (`LoadFromDB`). Ошибки восстановления логируются как предупреждения — сервис продолжает работу.

---

## Разработка

### Требования

- Go 1.26.2+
- Docker (для локальной БД)
- `GITHUB_TOKEN` для приватных модулей `github.com/pulsoats/*`

### Локальный запуск БД

```bash
docker compose up postgres -d
docker compose run --rm migrate
```

### Сборка

```bash
go build ./cmd
```

### Линтер

```bash
go vet ./...
```

### Структура приватных модулей

| Модуль | Назначение |
|--------|-----------|
| `github.com/pulsoats/core` | Общие утилиты: TLS-конфиг, errorsx, tlsconfig |
| `github.com/pulsoats/contracts` | Protobuf-контракты для gRPC |

Для сборки нужен `GITHUB_TOKEN` с правом `read:packages` / `contents` на организацию `pulsoats`.

```bash
export GITHUB_TOKEN=ghp_...
go env -w GOPRIVATE=github.com/pulsoats/*
```
