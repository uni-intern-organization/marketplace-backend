# Полная спецификация бэкенда для фронтенда и документация разработки

Документ можно копировать и передавать фронтенд-разработчику. В нём: что реализовано на бэкенде, как устроен API, как фронту с ним работать.

---

# Часть 1. Обзор бэкенда (для объяснения разработки)

## Стек и окружение

- **Язык:** Go 1.22  
- **БД:** PostgreSQL 16  
- **Хранилище файлов:** MinIO (S3-совместимое)  
- **Контейнеры:** Docker, docker-compose  
- **Порт API:** 8081  
- **База URL:** `http://localhost:8081` (в проде — свой домен)

## Архитектура

- **RBAC:** три роли — `student`, `recruiter`, `admin`. Доступ к эндпоинтам по ролям.
- **Авторизация:** JWT в заголовке `Authorization: Bearer <token>`. Токен выдаётся при логине и при регистрации.
- **Шифрование:** чувствительные поля (ФИО, телефон, сообщения, сопроводительные письма) хранятся в БД в зашифрованном виде (AES-256), расшифровка на бэкенде при отдаче.
- **Файлы:** резюме (PDF) и логотипы (PNG/JPG/WEBP) загружаются в MinIO, в БД хранится только ключ объекта. Скачивание — по presigned URL.

## Что реализовано

1. **Регистрация и вход**  
   Один эндпоинт регистрации с полем `role`, один эндпоинт логина. Повторная регистрация с тем же email запрещена (409).

2. **Профили**  
   Студент: ФИО, телефон, био, резюме (файл). Рекрутер: компания, ФИО, телефон, логотип (файл). Админ без профиля.

3. **Приглашения**  
   Рекрутер создаёт приглашение студенту (по `student_id`). Студент видит список приглашений и может принять/отклонить.

4. **Заявки**  
   Студент подаёт заявку рекрутеру (по `recruiter_id`, опционально по `invitation_id`). Рекрутер видит заявки и меняет статус: viewed / accepted / rejected.

5. **Поиск пользователей**  
   Админ и рекрутер: поиск по роли и префиксу email (рекрутер — только студенты).

6. **Файлы**  
   Загрузка резюме (multipart, поле `resume` или `file`, только PDF, до 5 МБ). Загрузка логотипа (multipart, поле `logo`). Получение ссылки на скачивание по `object_key`.

7. **Миграции**  
   SQL-миграции встроены в приложение, выполняются при старте (идемпотентные).

8. **CORS**  
   Настроен для запросов с браузера (в проде лучше ограничить по домену).

---

# Часть 2. API для фронтенда (полная спецификация)

## Base URL

- Разработка: `http://localhost:8081`  
- Прод: подставить свой (например `https://api.example.com`).

## Авторизация

- Защищённые запросы: заголовок  
  `Authorization: Bearer <JWT_TOKEN>`  
- Токен получают из ответа **POST /api/auth/login** или **POST /api/auth/register** (поле `token`).  
- Хранить токен на фронте (localStorage/sessionStorage/cookie) и подставлять в заголовок.  
- При 401 — перенаправлять на страницу входа.

## Роли и кто что может

| Роль       | Регистрация | Профиль        | Резюме | Логотип | Приглашения (создать) | Приглашения (список/принять) | Заявки (подать) | Заявки (список/статус) | Поиск пользователей |
|-----------|-------------|----------------|--------|---------|------------------------|------------------------------|-----------------|-------------------------|----------------------|
| student   | ✅          | свой (profile) | ✅     | —       | —                      | список, принять/отклонить    | подать          | свой список             | —                    |
| recruiter | ✅          | свой (recruiter)| —      | ✅      | создать                | свой список                  | —               | список, менять статус   | только студенты      |
| admin     | ✅          | нет            | —      | —       | —                      | —                            | —               | —                       | студенты и рекрутеры |

---

## Эндпоинты (список для фронта)

### Без токена

| Метод | URL | Назначение |
|--------|-----|------------|
| GET | `/health` | Проверка доступности API |
| POST | `/api/auth/register` | Регистрация (body: email, password, role) |
| POST | `/api/auth/login` | Вход (body: email, password) |

### С токеном (Authorization: Bearer &lt;token&gt;)

| Метод | URL | Кто | Назначение |
|--------|-----|-----|------------|
| GET | `/api/me` | все | Текущий пользователь + профиль |
| PUT / PATCH | `/api/me/profile` | student | Обновить профиль студента |
| PUT / PATCH | `/api/me/recruiter` | recruiter | Обновить профиль рекрутера |
| GET | `/api/users?id=<uuid>` | all | Получить пользователя по ID |
| POST | `/api/files/resume` | student | Загрузить резюме (multipart: file/resume) |
| POST | `/api/files/logo` | recruiter | Загрузить логотип (multipart: logo) |
| GET | `/api/files/url?key=<key>` | all | Ссылка на скачивание файла |
| POST | `/api/invitations` | recruiter | Создать приглашение студенту |
| GET | `/api/invitations` | student, recruiter | Список приглашений |
| PATCH | `/api/invitations?id=<uuid>` | student | Принять/отклонить приглашение |
| POST | `/api/applications` | student | Подать заявку |
| GET | `/api/applications` | student, recruiter | Список заявок |
| PATCH | `/api/applications?id=<uuid>` | recruiter | Изменить статус заявки |
| GET | `/api/search/users?role=&email=` | admin, recruiter | Поиск пользователей |

---

## Форматы запросов и ответов

### 1. POST /api/auth/register

- **Content-Type:** `application/json`
- **Body:**
```json
{
  "email": "user@example.com",
  "password": "password123",
  "role": "student"
}
```
- **role:** строго `"student"` | `"recruiter"` | `"admin"`.
- **Ответ 200:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "role": "student"
  }
}
```
- **Ответ 409:** пользователь с таким email уже есть — показать сообщение «Уже зарегистрирован».

---

### 2. POST /api/auth/login

- **Content-Type:** `application/json`
- **Body:**
```json
{
  "email": "user@example.com",
  "password": "password123"
}
```
- **Ответ 200:** такой же как у register (`token`, `user`).  
- **Ответ 401:** неверный email или пароль.

---

### 3. GET /api/me

- **Ответ 200:**
```json
{
  "user_id": "uuid",
  "email": "user@example.com",
  "role": "student",
  "profile": {
    "full_name": "Иван Иванов",
    "phone": "+79001234567",
    "bio": "Текст",
    "resume_url": "resumes/.../file.pdf"
  }
}
```
- Для рекрутера в `profile`: `company_name`, `full_name`, `phone`, `logo_url`.  
- Если профиль не заполнялся — поля в `profile` пустые или null.  
- **401:** нет или неверный токен.

---

### 4. PUT /api/me/profile (студент)

- **Content-Type:** `application/json`
- **Body:**
```json
{
  "full_name": "Иван Иванов",
  "phone": "+79001234567",
  "bio": "О себе"
}
```
- Можно отправлять не все поля (частичное обновление).  
- **Ответ 200:** `{"status":"ok"}`. **403:** не студент.

---

### 5. PUT /api/me/recruiter (рекрутер)

- **Content-Type:** `application/json`
- **Body:**
```json
{
  "company_name": "ООО Компания",
  "full_name": "Петр Петров",
  "phone": "+79007654321"
}
```
- **Ответ 200:** `{"status":"ok"}`. **403:** не рекрутер.

---

### 6. GET /api/users?id=&lt;uuid&gt;

- **Ответ 200:** `{"id":"uuid","email":"...","role":"..."}`.  
- **403:** студент/рекрутер запрашивают не себя. **404:** пользователь не найден.

---

### 7. POST /api/files/resume (студент)

- **Content-Type:** `multipart/form-data`
- **Поле формы:** имя `resume` или `file`, тип — файл (только PDF, до 5 МБ).
- **Ответ 200:** `{"object_key":"resumes/.../....pdf"}`.  
- Ссылку для скачивания получать через GET `/api/files/url?key=<object_key>`.

---

### 8. POST /api/files/logo (рекрутер)

- **Content-Type:** `multipart/form-data`
- **Поле формы:** имя `logo`, тип — файл (PNG/JPG/JPEG/WEBP, до 2 МБ).
- **Ответ 200:** `{"object_key":"logos/.../....png"}`.

---

### 9. GET /api/files/url?key=&lt;object_key&gt;

- **Ответ 200:** `{"url":"https://..."}` — временная ссылка на скачивание.  
- Фронт может использовать этот `url` для отображения/скачивания резюме или логотипа.

---

### 10. POST /api/invitations (рекрутер)

- **Content-Type:** `application/json`
- **Body:**
```json
{
  "student_id": "uuid-студента",
  "message": "Текст приглашения"
}
```
- **student_id:** взять из GET /api/search/users (рекрутер ищет студентов) или из ответа регистрации.  
- **Ответ 200:** объект приглашения с полями `id`, `recruiter_id`, `student_id`, `message`, `status`, `created_at`.  
- **409:** приглашение этому студенту уже отправлено.

---

### 11. GET /api/invitations

- **Ответ 200:** массив приглашений. У каждого есть `id` (нужен для PATCH).  
- Студент видит приглашения себе, рекрутер — отправленные им.

---

### 12. PATCH /api/invitations?id=&lt;invitation_uuid&gt; (студент)

- **Content-Type:** `application/json`
- **Body:** `{"status":"accepted"}` или `{"status":"declined"}`.  
- **id** в URL — из GET /api/invitations (поле `id`).

---

### 13. POST /api/applications (студент)

- **Content-Type:** `application/json`
- **Body:**
```json
{
  "recruiter_id": "uuid-рекрутера",
  "invitation_id": "uuid-приглашения",
  "cover_letter": "Текст письма"
}
```
- **invitation_id** можно не передавать.  
- **Ответ 200:** объект заявки с полями `id`, `student_id`, `recruiter_id`, `cover_letter`, `status`, `created_at`.

---

### 14. GET /api/applications

- **Ответ 200:** массив заявок. У каждой есть `id` (для PATCH рекрутером).

---

### 15. PATCH /api/applications?id=&lt;application_uuid&gt; (рекрутер)

- **Content-Type:** `application/json`
- **Body:** `{"status":"viewed"}` | `{"status":"accepted"}` | `{"status":"rejected"}`.  
- **id** в URL — из GET /api/applications.

---

### 16. GET /api/search/users

- **Query:** `role=student` или `role=recruiter`, `email=префикс` (опционально).  
- Рекрутер может только `role=student`.  
- **Ответ 200:** массив `[{ "id", "email", "role" }, ...]`.  
- По `id` можно подставлять `student_id` в приглашения или отображать список.

---

# Часть 3. Сценарии для фронта (как связать экраны с API)

## Регистрация и вход

1. Форма: email, пароль, выбор роли (student / recruiter / admin).  
2. POST `/api/auth/register` с этими данными.  
3. При 200 — сохранить `token` и `user` (например в store), перейти в личный кабинет.  
4. При 409 — показать «Email уже занят».  
5. Форма входа: email, пароль → POST `/api/auth/login` → сохранить токен и пользователя.

## Личный кабинет после входа

1. GET `/api/me` с токеном.  
2. По `role` показать разный интерфейс: студент / рекрутер / админ.  
3. В блоке профиля вывести `profile` (имя, телефон, био, ссылку на резюме/логотип через `/api/files/url?key=...`).

## Профиль студента

1. Форма: ФИО, телефон, био.  
2. PUT `/api/me/profile` с JSON.  
3. Загрузка резюме: форма с полем файла (имя `file` или `resume`), POST `/api/files/resume` (multipart).  
4. После успеха обновить данные на экране (повторный GET `/api/me` или обновить store).

## Профиль рекрутера

1. Форма: компания, ФИО, телефон.  
2. PUT `/api/me/recruiter` с JSON.  
3. Загрузка логотипа: поле `logo`, POST `/api/files/logo` (multipart).  
4. Показ логотипа: GET `/api/files/url?key=<logo_url из profile>`.

## Приглашения (рекрутер)

1. Поиск студентов: GET `/api/search/users?role=student&email=...`.  
2. Выбор студента → форма с текстом приглашения → POST `/api/invitations` с `student_id` и `message`.  
3. Список отправленных: GET `/api/invitations`.

## Приглашения (студент)

1. GET `/api/invitations` — список входящих.  
2. По каждой — кнопки «Принять» / «Отклонить».  
3. PATCH `/api/invitations?id=<id>` с `{"status":"accepted"}` или `{"status":"declined"}`.

## Заявки (студент)

1. Выбор рекрутера (из приглашений или поиска).  
2. Форма: сопроводительное письмо, опционально привязка к приглашению.  
3. POST `/api/applications` с `recruiter_id`, `cover_letter`, при необходимости `invitation_id`.  
4. Список своих заявок: GET `/api/applications`.

## Заявки (рекрутер)

1. GET `/api/applications` — список входящих заявок.  
2. По каждой — действия: «Просмотрено», «Принять», «Отклонить».  
3. PATCH `/api/applications?id=<id>` с `{"status":"viewed"}` / `"accepted"` / `"rejected"`.

## Поиск (админ / рекрутер)

1. Форма: роль (рекрутер — только student), опционально префикс email.  
2. GET `/api/search/users?role=...&email=...`.  
3. Таблица/список: id, email, role. Для рекрутера — использование id в приглашениях.

---

# Часть 4. Технические моменты для фронта

- **Ошибки:** тело ответа при 4xx/5xx — JSON с полем `error`, например `{"error":"invalid credentials"}`. Показывать пользователю.  
- **Токен:** при каждом запросе к защищённому эндпоинту добавлять заголовок `Authorization: Bearer <token>`.  
- **Файлы:** не отправлять файлы в JSON; только multipart/form-data с полями `file`/`resume` или `logo`.  
- **UUID в URL:** для PATCH приглашений и заявок брать `id` из ответов POST или из списков GET.  
- **CORS:** с текущего бэкенда запросы с браузера с другого порта/домена разрешены; в проде лучше ограничить `AllowedOrigins`.

---

Этого достаточно, чтобы фронтенд полностью соответствовал бэкенду: можно копировать документ целиком или части и использовать как спецификацию для разработки и объяснения полной разработки бэкенда.
