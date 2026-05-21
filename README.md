# Delivery Tracker — масштабируемый микросервис отслеживания доставки

Учебная реализация распределённой системы доставки заказов с real-time
трекингом курьера на карте.

> Выпускная квалификационная работа бакалавра, МАИ.

---

## Что система умеет

**Клиент:**
- регистрируется (роль `user`) и авторизуется (JWT + refresh-token)
- собирает заказ из каталога блюд, указывает адрес доставки на карте
  (Leaflet + OpenStreetMap), оплачивает наличными или картой курьеру
- видит список своих заказов с фильтрацией по статусу
- открывает трекинг — на карте в реальном времени движется курьер,
  показывается ETA, индикатор состояния WebSocket-соединения
- после доставки оставляет оценку 1–5 звёзд

**Курьер:**
- регистрируется (роль `courier`), включает/выключает статус «онлайн»
- видит ленту доступных заказов с превью состава, принимает заказ
  вручную («Подробнее» → «Принять»)
- ведёт активную доставку по конечному автомату:
  `assigned → picked_up → in_transit → delivered`
- эмулирует GPS в браузере (без отдельного железа) для демонстрации

**Система:**
- асинхронные доменные события через Kafka
  (`orders.created`, `delivery.assigned`, `delivery.status`)
- метрики Prometheus + дашборды Grafana
- circuit breaker и retry с экспоненциальным backoff
- rate limiting на API Gateway (Redis-backed)
- spatial-запросы PostGIS для проверки попадания в зону доставки

---

## Технологический стек

### Backend
- **Go 1.23+** — основной язык
- **gRPC** + **grpc-gateway** — внутренний RPC и автогенерируемый REST из `.proto`
- **PostgreSQL 16 + PostGIS 3** — реляционные и пространственные данные
- **Redis 7** — кэш, refresh-токены, rate-limiting, Pub/Sub для WebSocket fan-out
- **Apache Kafka 3.7 (KRaft)** — асинхронные доменные события (`segmentio/kafka-go`)
- **chi** — HTTP-роутер, **pgx/v5** — драйвер PG, **slog** — structured logging
- **golang-jwt/v5** — JWT HS256, **bcrypt** — хеширование паролей (cost 12)
- **golang-migrate** — миграции, **gobreaker** — circuit breaker, **backoff/v4** — retry

### Frontend
- **React 19** + **TypeScript** + **Vite**
- **Tailwind v4** + **shadcn-ui** — стилизация и UI-примитивы
- **TanStack Query** — server state, **MSW** — моки в тестах
- **Leaflet** + **OpenStreetMap** + **Nominatim** — карта и геокодирование
- **Vitest** + **React Testing Library** — тесты

### Инфраструктура и наблюдаемость
- **Docker** + **Docker Compose** — оркестрация всех сервисов
- **Prometheus** — сбор метрик, **Grafana** — дашборды
- **k6** — нагрузочное и стресс-тестирование
- **Swagger UI** — интерактивная документация REST

---

## Быстрый старт

### Требования
- Docker и Docker Compose v2+
- Go 1.23+ (только если собираешь локально, для Docker-запуска не нужен)
- Make

### Запуск всей системы
```bash
git clone https://github.com/Nikita14ZSP/MAI_FQW-Delivery-Tracker.git
cd MAI_FQW-Delivery-Tracker
cp .env.example .env        # при желании поменять JWT_SECRET
make up-all                 # поднимет все 5 микросервисов + инфраструктуру
```

Первый запуск собирает Docker-образы и применяет миграции автоматически
(~3–5 минут). Когда все контейнеры `healthy` — система готова.

### Точки входа
| URL | Что это |
|---|---|
| `http://localhost` | Веб-приложение (клиент и курьер) |
| `http://localhost:8080/swagger/index.html` | Swagger UI всех REST-эндпоинтов |
| `http://localhost:3000` | Grafana (admin / admin) — дашборды метрик |
| `http://localhost:9090` | Prometheus — сырые метрики и таргеты |
| `http://localhost:8080/metrics` | Метрики api-gateway |

### Остановка и очистка
```bash
make down              # остановить контейнеры (данные сохраняются)
make down-volumes      # остановить и удалить тома (полный сброс БД)
```

### Прочие make-цели
```bash
make build             # go build всех сервисов
make test              # go test ./... -race
make proto             # regenerate protobuf (.pb.go, .pb.gw.go)
make migrate           # docker-применение миграций
make test-smoke        # k6 smoke-тест
make test-load         # k6 нагрузочный сценарий
make test-stress       # k6 стресс-сценарий
make clean-loadtest    # очистка БД после нагрузочных запусков
```

---

## Структура проекта

```
.
├── proto/                          # .proto контракты (single source of truth)
│   ├── common/  delivery/  menu/
│   ├── notification/  order/  tracking/
│
├── gen/                            # сгенерированный код gRPC + grpc-gateway
│
├── services/                       # пять микросервисов, общая структура
│   ├── api-gateway/                #   точка входа: REST/WS-прокси, RBAC,
│   │                               #   rate-limit, JWT, Swagger
│   ├── order-service/              #   auth, меню, заказы
│   ├── delivery-service/           #   доставки, зоны, рейтинги, курьеры
│   ├── tracking-service/           #   WebSocket-хаб, GPS, Redis Pub/Sub
│   └── notification-service/       #   подписка на Kafka, лента уведомлений
│
│       Внутри каждого сервиса:
│       ├── cmd/main.go             #   точка входа, wiring зависимостей
│       ├── internal/
│       │   ├── domain/             #     модели и инварианты
│       │   ├── repository/         #     PostgreSQL через pgx
│       │   ├── service/            #     бизнес-логика
│       │   ├── handler/grpc/       #     gRPC-обработчики
│       │   ├── handler/http/       #     HTTP-обработчики (где есть)
│       │   └── kafka/              #     producers и consumers событий
│       └── migrations/             #   SQL-миграции golang-migrate
│
├── pkg/                            # переиспользуемые библиотеки
│   ├── config/    errors/    grpclient/
│   ├── health/    kafka/     logger/
│   ├── metrics/   middleware/  postgres/   redis/
│
├── frontend/                       # React SPA
│   └── src/
│       ├── pages/                  #   страницы (login, orders, tracking, courier)
│       ├── components/
│       │   ├── auth/  orders/  tracking/  courier/  ui/
│       ├── hooks/                  #   useAuth, useOrderTracking, useGpsSimulator
│       ├── lib/
│       │   ├── api/                #   axios-клиенты по доменам
│       │   ├── schemas/            #   zod-схемы (зеркало proto)
│       │   └── utils/
│       ├── contexts/               #   AuthContext
│       ├── router/                 #   ProtectedRoute, RoleRedirect
│       └── test/                   #   MSW + setup
│
├── configs/                        # конфиги Prometheus, Grafana
├── scripts/                        # обслуживающие SQL-скрипты
├── tests/k6/                       # сценарии нагрузочного тестирования
├── third_party/                    # google/api proto для grpc-gateway
├── tools/                          # генераторы (buf, protoc plugins)
│
├── docker-compose.yml              # все 5 сервисов + PG + Redis + Kafka + observability
├── Makefile                        # build/test/up/migrate/proto/k6 цели
└── go.mod
```

---

## Безопасность

- JWT HS256, access-token TTL 15 мин, refresh-token TTL 7 дней с
  атомарной ротацией через Redis `GETDEL`
- bcrypt для паролей (cost 12)
- RBAC через middleware на api-gateway (per-route правила в `RBACRules`)
- Rate-limiting на основе Redis `INCR`+`EXPIRE`
- Раздельная rate-limit политика для GPS-обновлений (1 запрос / 5 сек на курьера)
- Single-use WebSocket-ticket с claim `purpose=ws` (60 с TTL) — основные
  access-токены не попадают в URL и логи прокси
- Все секреты вынесены в `.env` (см. `.env.example`)

---

## Наблюдаемость

- `/metrics` на каждом сервисе → Prometheus → Grafana
- Структурированные логи (slog) с полем `service` и `trace_id`
- Healthcheck'и (`/health`) у каждого контейнера в Docker Compose
- Метрики gRPC, HTTP и Kafka publish/consume

---

## Лицензия

Проект разработан в учебных целях в рамках ВКР МАИ.
