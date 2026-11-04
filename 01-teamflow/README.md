# 01. TeamFlow — Таск-трекер

Версия v0.1: Проверка циклов зависимостей; блокировка завершения зависимых задач; optimistic locking; журнал изменений; фильтрация и keyset-пагинация.

## Запуск

Из корня этой папки:

```bash
python3 scripts/setup.py
docker compose up --build -d app
python3 scripts/demo.py teamflow
```

HTTP: `http://localhost:8080`. Нужны Docker Compose v2 и Python 3. `.env` генерируется локально и исключён из Git. Остановка: `docker compose down`.

## Файлы

- `cmd/teamflow` — точка входа сервиса.
- `projects/teamflow` — бизнес-логика, описание API и OpenAPI.
- `internal/platform` — собственная копия инфраструктурного кода этого репозитория.
- `migrations` — общие служебные таблицы и схема только этого проекта.
- `tests` — интеграционные тесты именно этого проекта.
- `.github/workflows` — CI и ручной CD именно этого проекта.

## Проверка

```bash
go test ./...
go vet ./...
go build ./...
```

```bash
git init
git add .
git commit -m "Initial version of TeamFlow"
git branch -M main
git remote add origin https://github.com/YOUR_USERNAME/01-teamflow.git
git push -u origin main
```
