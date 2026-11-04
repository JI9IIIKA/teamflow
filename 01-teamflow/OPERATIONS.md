# Эксплуатация TeamFlow

API_KEYS_JSON генерируется setup.py: admin, editor, viewer; workspace demo. HTTP_ADDR внутри контейнера — :8080, порт хоста — 8080. Redis в Compose ограничивает запросы до 600/мин на tenant. PostgreSQL — источник истины. Health/readiness: /healthz, /readyz.

Для запуска без Docker задайте DATABASE_URL, экспортируйте API_KEYS_JSON из .env и выполните `go run ./cmd/teamflow`. Отдельное применение миграций: `go run ./cmd/teamflow migrate`.

## События

```bash
BROKER=rabbitmq docker compose --profile rabbit --profile consumer up --build -d
# Или Kafka:
BROKER=kafka docker compose --profile kafka --profile consumer up --build -d
```

Relay отправляет доменные события из outbox, consumer сохраняет их в event_receipts. Доставка at-least-once с дедупликацией. SQL-очереди бизнес-воркеров брокером не заменяются. Список реально реализованных событий определяется вызовами Request.Event в коде проекта; если проект их не создаёт, relay не получает работы.

## Kubernetes

Chart deploy/helm разворачивает один образ. БД/Redis/брокеры подключаются отдельно. Создайте Secret teamflow-runtime с DATABASE_URL, API_KEYS_JSON и необходимыми дополнительными переменными. Затем укажите образ и этот Secret при helm upgrade --install. Ручной CD требует staging environment и KUBECONFIG_B64. Внешний TLS настраивается на ingress. Ни репозиторий, ни образ, ни сайт автоматически не публиковались.

## Надёжность

Мутации сериализуются advisory lock по tenant+project; воркеры используют блокировки строк. Idempotency-Key хранит успешный результат вместе с транзакцией; ключ с другим содержимым запроса возвращает 409. Очистка истории и ключей пока ручная. Compose предназначен для локального запуска.
