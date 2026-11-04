# TeamFlow — Командный таск-трекер

Проект №01. Самостоятельное Go-приложение. Сборка: `./cmd/teamflow`.
Общие пакеты находятся в `internal/platform`.

## Реализовано в v0.1

Проверка циклов зависимостей; блокировка завершения зависимых задач; optimistic locking; журнал изменений; фильтрация и keyset-пагинация.

Данные: `tasks, task_dependencies, task_history`. Схема: `../../migrations/teamflow_001.sql`.
Все записи изолированы по tenant; роль определяется серверным API-токеном.

## Запуск

Из корня репозитория:

```bash
python3 scripts/setup.py  # один раз для всего набора
PROJECT=teamflow docker compose up --build -d app
python3 scripts/demo.py teamflow
```

Для JobRunner запускайте ещё `--profile jobs`; точные команды профилей — в корневом README.
Демо создаёт тестовые записи. Остановить: `docker compose down`.

## API

Все пути требуют `Authorization: Bearer <token>`. Ошибки возвращаются как JSON.
`Idempotency-Key` необязателен, но рекомендуется для команд создания и отправки.
Повтор того же запроса с тем же ключом возвращает сохранённый ответ; изменение запроса даёт 409.

| Метод | Путь |
|---|---|
| POST | `/tasks` |
| GET | `/tasks` |
| GET | `/tasks/{id}` |
| PATCH | `/tasks/{id}` |
| POST | `/tasks/{id}/dependencies` |
| GET | `/tasks/{id}/history` |
| DELETE | `/tasks/{id}` |

## Примеры тел запросов

`POST /tasks`:

```json
{
  "title": "Задача"
}
```

`PATCH /tasks/{id}`:

```json
{
  "title": "Задача",
  "status": "done",
  "version": 1
}
```

`POST /tasks/{id}/dependencies`:

```json
{
  "depends_on": "TASK_ID"
}
```

## Проверка

```bash
TEST_DATABASE_URL='postgres://portfolio:portfolio-local-only@localhost:5432/portfolio?sslmode=disable' \
  go test -count=1 -v ./tests -run '^TestTeamFlow$'
```

Интеграционные тесты пишут данные в указанную БД. Используйте отдельную тестовую базу.
Полный сценарий проверки находится в `../../tests/integration_test.go`.

## Границы версии и дальнейшая работа

Повторяющиеся задачи, отдельные проекты внутри workspace, фоновые напоминания.

Это реализованное ядро проекта, а не заявление о завершении всех расширений исходной идеи.
Подробнее об ограничениях окружения проверки — `../../VALIDATION.md`.
