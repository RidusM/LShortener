# LSHortener 

**LSHortener** — высокопроизводительный сервис для сокращения URL с расширенной аналитикой, построенный на Go. Предназначен для быстрой генерации коротких ссылок, отслеживания переходов и гибкого управления временем жизни URL.

---

## Содержание

- [Возможности](#возможности)
- [Архитектура](#архитектура)
- [Быстрый старт](#быстрый-старт)
- [Конфигурация](#конфигурация)
- [API](#api)
- [Разработка](#разработка)
- [Структура проекта](#структура-проекта)
- [Схема БД](#схема-бд)

---

## Возможности

* ⚡ **Мгновенные редиректы** — кэширование в Redis обеспечивает субмиллисекундный ответ
* 📊 **Детальная аналитика** — переходы по дням, User-Agent, источникам (Referer), последние клики и общий счётчик
* 🔗 **Кастомные алиасы** — поддержка пользовательских коротких кодов с проверкой уникальности и резервных слов
* ⏰ **Истечение срока действия** — настройка TTL для ссылок на уровне запроса или глобально
* 🔄 **Асинхронная запись кликов** — редирект не блокируется ожиданием записи в БД
* 🛡️ **Надёжность** — UUID v7, транзакции, валидация ввода, graceful shutdown, structured logging
* 🎨 **Готовый веб-интерфейс** — современный русскоязычный UI с поддержкой мобильных устройств

---

## Архитектура

```
HTTP-запрос
    │
    ▼
ShortenerHandler (Gin)
    │
    ▼
ShortenerService (Бизнес-логика)
    │
    ├── URLRepository          (PostgreSQL) — хранение ссылок
    ├── AnalyticsRepository    (PostgreSQL) — аналитика кликов
    └── CacheRepository        (Redis)      — кэш для быстрых редиректов
```

Проект следует **Clean Architecture**, обеспечивая строгое разделение ответственности:

```
cmd/lshortener          → Точка входа, инициализация контекста и graceful shutdown
internal/app            → Сборка графа зависимостей (DI), запуск компонентов
internal/config         → Парсинг и валидация конфигурации из .env
internal/entity         → Доменные модели (URL, Analytics, ошибки)
internal/service        → Бизнес-логика, транзакции, кэш-стратегии, валидация
internal/repository     → Инфраструктурный слой (PostgreSQL, Redis)
internal/transport/http → Gin handlers, middleware, маршруты, Swagger
web/                    → Статический фронтенд (index.html, expired.html)
migrations/             → SQL-миграции схемы БД
```

---

## Быстрый старт

### Требования

- [Docker](https://docs.docker.com/get-docker/) и Docker Compose
- Go 1.24+ (только для локальной разработки без Docker)

### Запуск через Docker Compose (рекомендуется)

```bash
# 1. Клонировать репозиторий
git clone https://github.com/RidusM/lshortener
cd lshortener

# 2. Скопировать конфиг и при необходимости отредактировать
cp .env.example .env  # если есть, или использовать существующий .env

# 3. Запустить всё (БД + Redis + приложение + миграции)
make compose-up
```

Сервис будет доступен на `http://localhost:8080`.

### Запуск только инфраструктуры (для локальной разработки)

```bash
# Поднять БД, Redis, RabbitMQ
make infra-up

# Применить миграции
make migrate-up

# Запустить приложение
make run
```

### Остановка и очистка

```bash
# Остановить все контейнеры и удалить volumes
make compose-down
```

---

## Конфигурация

Все параметры задаются через переменные окружения (файл `.env`).  
Скопируйте `.env.example` в `.env` и заполните нужные значения.

### Приложение

| Переменная    | По умолчанию       | Описание                                     |
|---------------|--------------------|----------------------------------------------|
| `APP_NAME`    | `delayed-notifier` | Название сервиса (используется в логах)      |
| `APP_VERSION` | `1.0.0`            | Версия                                       |
| `ENV`         | `local`            | Окружение: `local`, `dev`, `staging`, `prod` |

### Сервис (логика retry)

| Переменная                  | По умолчанию            | Описание                                                 |
|-----------------------------|-------------------------|-----------------------------------------------------|
| `SERVICE_QUERY_LIMIT`       | `10`                    | Лимит записей в аналитике                         |
| `SERVICE_RETRY_DELAY`       | `5m`                    | Задержка между попытками генерации                      |
| `SERVICE_MAX_RETRIES`       | `3`                     | Макс. попыток генерации уникального кода                           |
| `SERVICE_BASE_URL`          | `http://localhost:8080` | Базовый URL для генерации коротких ссылок |
| `SERIVCE_SHORT_CODE_LENGTH` | `6`                     | Длина автогенерируемого кода |
| `SERVICE_DEFAULT_TTL`       | `0`                     | Время жизни ссылки по умолчанию (0 = бесконечно)                   |

### База данных

| Переменная           | По умолчанию                                                    |
|----------------------|-----------------------------------------------------------------|
| `DB_DSN`             | `postgres://postgres:postgres@db:5432/notify_db?sslmode=disable` |
| `DB_POOL_MAX`        | `20`                                                            |
| `DB_CONN_ATTEMPTS`   | `5`                                                             |
| `DB_BASE_RETRY_DELAY` | `100ms` |
| `DB_MAX_RETRY_DELAY`  | `5s`    | 

### Redis

| Переменная      | По умолчанию     |
|-----------------|------------------|
| `CACHE_ADDR`    | `redis:6379`     |
| `CACHE_PASSWORD`| _(пусто)_        |
| `CACHE_DB`      | `0`              |
| `CACHE_DIAL_TIMEOUT` | `5s`        |
| `CACHE_READ_TIMEOUT` | `3s`        |
| `CACHE_WRITE_TIMEOUT` | `3s`       |
| `CACHE_POOL_SIZE`     | `20`       |

### HTTP-сервер

| Переменная                 | По умолчанию |
|----------------------------|--------------|
| `HTTP_HOST`                | `0.0.0.0`    |
| `HTTP_PORT`                | `8080`       |
| `HTTP_READ_TIMEOUT`        | `5s`         |
| `HTTP_WRITE_TIMEOUT`       | `5s`         |
| `HTTP_IDLE_TIMEOUT`        | `60s`        |
| `HTTP_SHUTDOWN_TIMEOUT`    | `10s`        |
| `HTTP_READ_HEADER_TIMEOUT` | `5s`         |
| `HTTP_MAX_HEADER_BYTES`    | `1048576`    |

### Logger

| Переменная           | По умолчанию                  |
|----------------------|-------------------------------|
| `LOGGER_LEVEL`       | `info`                        |
| `LOGGER_FILENAME`    | `./logs/delayed-notifier.log` |
| `LOGGER_MAX_SIZE`    | `100`                         |
| `LOGGER_MAX_BACKUPS` | `3`                           |
| `LOGGER_MAX_AGE`     | `28`                          |

---

## API

Полная документация доступна в Swagger UI: `http://localhost:8080/swagger/index.html`

### `POST /shorten` — Создать короткую ссылку

Создаёт короткую ссылку для указанного оригинального URL.

```bash
curl -X POST http://localhost:8080/shorten \
  -H "Content-Type: application/json" \
  -d '{
    "original_url": "https://example.com/very-long-url",
    "custom_alias": "mylink",
    "expires_at": "2026-12-31T23:59:59Z"
  }'
```

**Ответ `201 Created`:**
```json
{
  "id": "019e03e9-28aa-7af5-bdab-ba9eab1b9fd9",
  "short_code": "mylink",
  "short_url": "http://localhost:8080/mylink",
  "original_url": "https://example.com/very-long-url",
  "custom_alias": "mylink",
  "expires_at": "2026-12-31T23:59:59Z",
  "created_at": "2026-05-07T19:04:03.693925Z"
}
```

**Возможные ошибки**

| Код   | Описание                                       |
|-------|------------------------------------------------|
| `400` | Неверный формат URL, невалидный alias или дата |
| `409` | Пользовательский alias уже занят               |
| `500` | Внутренняя ошибка сервера                      |


---

### `GET /{short_code}` — Редирект на оригинальный URL

Перенаправляет пользователя на исходную ссылку и записывает статистику перехода.

```bash
curl -I http://localhost:8080/mylink
# HTTP/1.1 301 Moved Permanently
# Location: https://example.com/very-long-url
```

**Ответ `301`**

Успешный редирект (заголовок `Location`)

**Возможные ошибки**

| Код   | Описание                          |
|-------|-----------------------------------|
| `400` | Не указан `short_code`            |
| `404` | Ссылка не найдена                 |
| `410` | Ссылка истекла или деактивирована |
| `500` | Внутренняя ошибка сервера         |

---

### `GET /{short_code}/analytics` — Получить аналитику

Возвращает детальную статистику переходов по короткой ссылке.

```bash
curl http://localhost:8080/mylink/analytics
```

**Ответ `200 OK`:**
```json
{
  "short_code": "mylink",
  "original_url": "https://example.com/very-long-url",
  "total_clicks": 42,
  "clicks_by_day": {
    "2026-05-07": 15,
    "2026-05-08": 27
  },
  "clicks_by_user_agent": {
    "Mozilla/5.0...": 38,
    "curl/7.68.0": 4
  },
  "recent_clicks": [
    {
      "user_agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
      "ip_address": "192.168.1.100",
      "referer": "https://google.com",
      "clicked_at": "2026-05-08T14:23:45Z"
    }
  ],
  "created_at": "2026-05-07T19:04:03.693925Z"
}
```

**Возможные ошибки**

| Код   | Описание                          |
|-------|-----------------------------------|
| `400` | Не указан `short_code`            |
| `404` | Ссылка не найдена                 |
| `500` | Внутренняя ошибка сервера         |

---

### `GET /expired` — Страница истёкшей ссылки

Отображает пользовательскую страницу, когда ссылка истекла. Доступна как статический HTML.

---

### `GET /health` — Проверка работоспособности

```bash
curl http://localhost:8080/health
# {"status":"ok","time":"2026-05-06T10:00:00Z"}
```

---

## Разработка

```bash
# Установить инструменты разработки (линтеры, генераторы)
make install-tools

# Запустить юнит-тесты с race detector и покрытием (wip)
make test

# Запустить интеграционные тесты (требует Docker) (wip)
make integration-test

# Запустить все тесты (wip)
make test-all

# Запустить линтер (golangci-lint)
make lint

# Отформатировать код (gofumpt, gci, goimports, golines)
make format

# Сгенерировать Swagger-документацию
make swagger

# Собрать бинарник для linux/amd64
make build

# Собрать локально под текущую ОС
make build-local

# Собрать Docker-образ
make build-docker

# Проверить зависимости на уязвимости (требует govulncheck)
make deps-audit

# Выполнить все пре-коммит проверки
make pre-commit

# Очистить артефакты сборки и моки
make clean

# Очистить кэш тестов и линтера
make clean-cache
```

---

## Структура проекта

```
lshortener/
├── cmd/
│   └── lshortener/
│       └── main.go              # Точка входа, graceful shutdown
├── configs/                     # Конфигурационные файлы (.env)
├── docs/                        # Swagger-документация (генерируется)
├── internal/
│   ├── app/
│   │   └── app.go               # Инициализация и запуск всех компонентов
│   ├── config/
│   │   └── config.go            # Парсинг конфигурации из env-переменных
│   ├── entity/                  # Доменные модели: URL, Analytics, ошибки
│   ├── repository/              # Реализации репозиториев:
│   │   ├── url.go               # PostgreSQL: работа с таблицей urls
│   │   ├── analytics.go         # PostgreSQL: аналитика кликов
│   │   └── cache.go             # Redis: кэширование ссылок
│   ├── service/                 # Бизнес-логика:
│   │   └── service.go           # Создание, редирект, аналитика, валидация
│   └── transport/http/          # HTTP-слой (Gin):
│       ├── handlers.go          # Обработчики запросов
│       ├── routes.go            # Маршрутизация + Swagger
│       ├── middleware.go        # Middleware: logging, CORS, request ID
│       ├── http_server.go       # Настройка и запуск HTTP-сервера
│       └── shortener_transport.go # Интерфейсы и инициализация
├── migrations/                  # SQL-миграции (golang-migrate)
│   ├── 00000001_create_table_urls.up.sql
│   └── 00000002_create_table_analytics.up.sql
├── web/
│   ├── index.html               # Веб-интерфейс (сокращение ссылок)
│   └── expired.html             # Страница истёкшей ссылки
├── pkg/
│   └── keygen/
│       └── keygen.go            # Генератор Base62 коротких кодов
├── docker-compose.yml           # Оркестрация: app, db, redis, migrator
├── Dockerfile                   # Multi-stage сборка
├── Makefile                     # Автоматизация задач разработки
└── .env                         # Переменные окружения
```

---

## Схема БД

```sql
-- Таблица ссылок
CREATE TABLE IF NOT EXISTS urls (
    id UUID PRIMARY KEY,
    short_code VARCHAR(20) NOT NULL UNIQUE,
    original_url TEXT NOT NULL,
    custom_alias VARCHAR(50) UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    click_count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Индексы для быстрого поиска
CREATE INDEX IF NOT EXISTS idx_urls_short_code ON urls (short_code);
CREATE INDEX IF NOT EXISTS idx_urls_custom_alias ON urls (custom_alias) WHERE custom_alias IS NOT NULL;

-- Таблица аналитики кликов
CREATE TABLE IF NOT EXISTS analytics (
    id UUID PRIMARY KEY,
    url_id UUID NOT NULL REFERENCES urls(id) ON DELETE CASCADE,
    user_agent TEXT NOT NULL,
    ip_address INET NOT NULL,
    referer TEXT,
    clicked_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Индексы для аналитических запросов
CREATE INDEX IF NOT EXISTS idx_analytics_url_id ON analytics (url_id);
CREATE INDEX IF NOT EXISTS idx_analytics_clicked_at ON analytics (clicked_at);
CREATE INDEX IF NOT EXISTS idx_analytics_url_id_clicked_at ON analytics (url_id, clicked_at DESC);
```