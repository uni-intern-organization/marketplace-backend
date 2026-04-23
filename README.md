# Marketplace Backend

Бэкенд маркетплейса для студентов и рекрутеров: Go, PostgreSQL, MinIO (S3), JWT, RBAC, AES-256 шифрование данных.

## Роли

- **Student** — регистрация, профиль, загрузка резюме (PDF), отклики на приглашения, подача заявок.
- **Recruiter** — профиль компании, логотип, приглашение студентов, просмотр/принятие/отклонение заявок, поиск студентов.
- **Admin** — поиск пользователей, доступ к данным по необходимости.

## Стек

- Go 1.22, PostgreSQL 16, MinIO (S3-совместимое хранилище)
- JWT-авторизация, AES-256 для чувствительных полей (ФИО, телефон, сообщения)
- Docker и docker-compose, порт API: **8080**

## Запуск

### Локально (с .env)

```bash
cp .env.example .env
# Отредактируйте .env (DB_HOST=localhost, S3_ENDPOINT=localhost:9000)
go mod download
go run ./cmd/api
```

Предварительно должны быть запущены PostgreSQL и MinIO (или весь стек через docker-compose без сервиса api, см. ниже).

### Docker Compose

```bash
docker compose up -d
```

Сервисы:

- **API**: http://localhost:8080
- **PostgreSQL** (с хоста, pgAdmin): **localhost:5433** → в контейнере порт 5432 (user/pass: postgres/postgres, DB: marketplace)
- **MinIO**: http://localhost:9000 (minioadmin/minioadmin), консоль: http://localhost:9001

Миграции выполняются при старте API.

## API (кратко)

| Метод | Путь | Описание |
|-------|------|----------|
| POST | /api/auth/register | Регистрация (email, password, role: student \| recruiter \| admin). Повторная регистрация с тем же email запрещена. |
| POST | /api/auth/login | Вход, в ответе JWT. |
| GET | /api/me | Текущий профиль (по JWT). |
| PUT/PATCH | /api/me/profile | Обновление профиля студента (student). |
| PUT/PATCH | /api/me/recruiter | Обновление профиля рекрутера. |
| GET | /api/users?id=... | Получить пользователя по ID (self или admin). |
| POST | /api/files/resume | Загрузка PDF-резюме (student), multipart form "resume". |
| POST | /api/files/logo | Загрузка логотипа компании (recruiter), multipart form "logo". |
| GET | /api/files/url?key=... | Presigned URL для скачивания файла. |
| POST | /api/invitations | Создать приглашение студенту (recruiter). |
| GET | /api/invitations | Список приглашений (student/recruiter). |
| PATCH | /api/invitations?id=... | Принять/отклонить приглашение (student): body `{"status":"accepted"\|"declined"}`. |
| POST | /api/applications | Подать заявку (student). |
| GET | /api/applications | Список заявок. |
| PATCH | /api/applications?id=... | Обновить статус заявки (recruiter): viewed/accepted/rejected. |
| GET | /api/search/users?role=student&email=... | Поиск пользователей (admin, recruiter — только студенты). |
| GET | /health | Проверка живости сервиса. |

Заголовок авторизации: `Authorization: Bearer <token>`.

## Переменные окружения

См. `.env.example`. Основные: `PORT`, `DB_*`, `JWT_SECRET`, `AES_KEY`, `S3_*`.

## Миграции

SQL-миграции лежат в `internal/db/migrations/` и встраиваются в бинарник. При старте приложения выполняются все `*.up.sql` по порядку.

## Деплой на GitHub

Репозиторий уже привязан к `origin`. Для пуша:

```bash
git add .
git commit -m "Backend: Go API, PostgreSQL, MinIO, JWT, RBAC, migrations"
git push -u origin main
```
