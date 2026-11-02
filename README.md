# LibraTrack

Вебзастосунок обліку бібліотеки на Go, Gin, PostgreSQL і HTMX.
Початкова версія Lesson 14 — завершений застосунок Lesson 10, коміт `b75b1c0a89936a14851c896d1afe856d653bb754`.

## Можливості

Каталог і пошук книг, браузерний вхід, ролі reader/librarian/admin,
REST API, Swagger та облік видач. HTML, CSS, JavaScript і SQL-міграції
вбудовані у виконуваний файл.

## Локальний запуск

Потрібні Go за версією `go.mod`, Docker Compose v2 та Python 3.

```bash
python3 scripts/setup_env.py
docker compose up -d --wait postgres
go run ./cmd/demo-users
go run ./cmd/api
```

Конфігурація й облікові дані генеруються локально та не входять до Git.
Параметри підключення наведено в `.env.example`.

## Перевірки

```bash
go build ./...
go vet ./...
go test -race -count=1 ./...
```

Інтеграційні тести потребують окремої PostgreSQL-бази з назвою `_test`.
Кожен тест створює власну схему. Секрети, журнали, скриншоти та звіти
не публікуються в репозиторії.
