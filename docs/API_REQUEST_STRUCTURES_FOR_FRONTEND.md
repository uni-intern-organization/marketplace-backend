# Структуры запросов API для фронтенда

Base URL: `http://localhost:8080`  
Защищённые запросы: заголовок `Authorization: Bearer <token>`  
Content-Type для JSON: `application/json`

---

## 1. Health (без авторизации)

**URL:** `http://localhost:8080/health`  
**Метод:** `GET`  
**Заголовки:** не нужны  
**Тело:** нет  

**Пример ответа (200):** `ok` (текст)

---

## 2. Регистрация

**URL:** `http://localhost:8080/api/auth/register`  
**Метод:** `POST`  
**Заголовки:** `Content-Type: application/json`  
**Тело запроса (JSON):**

```json
{
  "email": "admin@marketplace.local",
  "password": "AdminSecure#1",
  "role": "student"
}
```

| Поле      | Тип   | Обязательно | Описание                              |
|-----------|-------|-------------|----------------------------------------|
| email     | string| да          | Email пользователя                     |
| password  | string| да          | Пароль                                 |
| role      | string| да          | `"student"` \| `"recruiter"` \| `"admin"` |

**Пример ответа (200):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "admin@marketplace.local",
    "role": "admin"
  }
}
```

**Ошибки:** 409 — пользователь с таким email уже зарегистрирован.

---

## 3. Вход (логин)

**URL:** `http://localhost:8080/api/auth/login`  
**Метод:** `POST`  
**Заголовки:** `Content-Type: application/json`  
**Тело запроса (JSON):**

```json
{
  "email": "admin@marketplace.local",
  "password": "AdminSecure#1"
}
```

| Поле     | Тип   | Обязательно | Описание |
|----------|-------|-------------|----------|
| email    | string| да          | Email    |
| password | string| да          | Пароль   |

**Пример ответа (200):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "admin@marketplace.local",
    "role": "admin"
  }
}
```

**Ошибки:** 401 — неверный email или пароль.

---

## 4. Текущий пользователь и профиль

**URL:** `http://localhost:8080/api/me`  
**Метод:** `GET`  
**Заголовки:** `Authorization: Bearer <token>`  
**Тело:** нет  

**Пример ответа (200) — студент:**
```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "student@test.com",
  "role": "student",
  "profile": {
    "full_name": "Иван Иванов",
    "phone": "+79001234567",
    "bio": "О себе",
    "resume_url": "resumes/550e8400-.../file.pdf"
  }
}
```

**Пример ответа (200) — рекрутер:**
```json
{
  "user_id": "660e8400-e29b-41d4-a716-446655440001",
  "email": "recruiter@company.com",
  "role": "recruiter",
  "profile": {
    "company_name": "ООО Компания",
    "full_name": "Пётр Петров",
    "phone": "+79007654321",
    "logo_url": "logos/660e8400-.../logo.png"
  }
}
```

---

## 5. Обновить профиль студента

**URL:** `http://localhost:8080/api/me/profile`  
**Метод:** `PUT` или `PATCH`  
**Роль:** student  
**Заголовки:** `Authorization: Bearer <token>`, `Content-Type: application/json`  
**Тело запроса (JSON):**

```json
{
  "full_name": "Иван Иванов",
  "phone": "+79001234567",
  "bio": "Студент 3 курса, ищу стажировку"
}
```

| Поле      | Тип   | Обязательно | Описание |
|-----------|-------|-------------|----------|
| full_name | string| нет         | ФИО      |
| phone     | string| нет         | Телефон  |
| bio       | string| нет         | О себе   |

**Пример ответа (200):** `{"status": "ok"}`

---

## 6. Обновить профиль рекрутера

**URL:** `http://localhost:8080/api/me/recruiter`  
**Метод:** `PUT` или `PATCH`  
**Роль:** recruiter  
**Заголовки:** `Authorization: Bearer <token>`, `Content-Type: application/json`  
**Тело запроса (JSON):**

```json
{
  "company_name": "ООО Рога и Копыта",
  "full_name": "Пётр Петров",
  "phone": "+79007654321"
}
```

| Поле         | Тип   | Обязательно | Описание   |
|--------------|-------|-------------|------------|
| company_name | string| нет         | Компания   |
| full_name    | string| нет         | ФИО        |
| phone        | string| нет         | Телефон    |

**Пример ответа (200):** `{"status": "ok"}`

---

## 7. Получить пользователя по ID

Используется, в частности, когда рекрутер открывает профиль студента из заявки. Рекрутер и админ могут запрашивать любого пользователя; остальные — только себя.

**URL:** `http://localhost:8080/api/users?id=<uuid>`  
**Метод:** `GET`  
**Заголовки:** `Authorization: Bearer <token>`  
**Query-параметры:** `id` — UUID пользователя (например, student_id из заявки)  
**Тело:** нет  

**Пример запроса:** `GET http://localhost:8080/api/users?id=550e8400-e29b-41d4-a716-446655440000`

**Пример ответа (200) — пользователь-студент (возвращается полный профиль):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "student@test.com",
  "role": "student",
  "profile": {
    "full_name": "Иван Иванов",
    "phone": "+79001234567",
    "bio": "О себе",
    "resume_url": "resumes/550e8400-.../file.pdf",
    "skills": "Go, PostgreSQL, Docker",
    "education": "Университет, 3 курс",
    "experience_years": 0,
    "location": "Алматы",
    "availability": "remote"
  }
}
```

Для пользователя без профиля или не-студента поле `profile` отсутствует или пусто. При отсутствии пользователя с указанным id — **404** (фронт: «Кандидат не найден»).

---

## 8. Загрузить резюме (PDF)

**URL:** `http://localhost:8080/api/files/resume`  
**Метод:** `POST`  
**Роль:** student  
**Заголовки:** `Authorization: Bearer <token>`  
**Content-Type:** `multipart/form-data`  
**Тело запроса (form-data):**

| Имя поля | Тип  | Обязательно | Описание                    |
|----------|------|-------------|-----------------------------|
| resume   | file | да*         | PDF-файл, до 5 МБ           |
| file     | file | да*         | То же (альтернативное имя)  |

*Нужно одно из полей: `resume` или `file`.

**Пример ответа (200):**
```json
{
  "object_key": "resumes/550e8400-.../abc123.pdf"
}
```

Ссылку для скачивания получать через GET `/api/files/url?key=<object_key>`.

---

## 9. Загрузить логотип компании

**URL:** `http://localhost:8080/api/files/logo`  
**Метод:** `POST`  
**Роль:** recruiter  
**Заголовки:** `Authorization: Bearer <token>`  
**Content-Type:** `multipart/form-data`  
**Тело запроса (form-data):**

| Имя поля | Тип  | Обязательно | Описание              |
|----------|------|-------------|------------------------|
| logo     | file | да          | PNG/JPG/WEBP, до 2 МБ  |

**Пример ответа (200):**
```json
{
  "object_key": "logos/660e8400-.../logo.png"
}
```

---

## 10. Получить ссылку на скачивание файла

**URL:** `http://localhost:8080/api/files/url?key=<object_key>`  
**Метод:** `GET`  
**Заголовки:** `Authorization: Bearer <token>`  
**Query-параметры:** `key` — значение `object_key` из ответа загрузки резюме/логотипа  
**Тело:** нет  

**Пример запроса:** `GET http://localhost:8080/api/files/url?key=resumes/550e8400-.../abc123.pdf`

**Пример ответа (200):**
```json
{
  "url": "http://localhost:9000/marketplace/resumes/...?X-Amz-..."
}
```

Фронт использует `url` для отображения или скачивания файла.

---

## 11. Создать приглашение (рекрутер → студент)

**URL:** `http://localhost:8080/api/invitations`  
**Метод:** `POST`  
**Роль:** recruiter  
**Заголовки:** `Authorization: Bearer <token>`, `Content-Type: application/json`  
**Тело запроса (JSON):**

```json
{
  "student_id": "550e8400-e29b-41d4-a716-446655440000",
  "message": "Приглашаем вас на собеседование"
}
```

| Поле      | Тип   | Обязательно | Описание           |
|-----------|-------|-------------|--------------------|
| student_id| string| да          | UUID студента      |
| message   | string| нет         | Текст приглашения  |

**Пример ответа (200):**
```json
{
  "id": "inv-uuid-...",
  "recruiter_id": "660e8400-...",
  "student_id": "550e8400-...",
  "message": "Приглашаем вас на собеседование",
  "status": "pending",
  "created_at": "2025-02-23T12:00:00Z"
}
```

**Ошибки:** 409 — приглашение этому студенту уже отправлено.

---

## 12. Список приглашений

**URL:** `http://localhost:8080/api/invitations`  
**Метод:** `GET`  
**Роль:** student или recruiter  
**Заголовки:** `Authorization: Bearer <token>`  
**Тело:** нет  

**Пример ответа (200):** массив объектов:
```json
[
  {
    "id": "inv-uuid-...",
    "recruiter_id": "660e8400-...",
    "student_id": "550e8400-...",
    "message": "Текст приглашения",
    "status": "pending",
    "created_at": "2025-02-23T12:00:00Z"
  }
]
```

---

## 13. Принять / отклонить приглашение (студент)

**URL:** `http://localhost:8080/api/invitations?id=<uuid>`  
**Метод:** `PATCH`  
**Роль:** student  
**Заголовки:** `Authorization: Bearer <token>`, `Content-Type: application/json`  
**Query-параметры:** `id` — UUID приглашения (из GET /api/invitations)  
**Тело запроса (JSON):**

```json
{
  "status": "accepted"
}
```

или

```json
{
  "status": "declined"
}
```

| Поле   | Тип   | Обязательно | Описание                    |
|--------|-------|-------------|-----------------------------|
| status | string| да          | `"accepted"` \| `"declined"` |

**Пример запроса:** `PATCH http://localhost:8080/api/invitations?id=inv-uuid-...`  
**Пример ответа (200):** `{"status": "accepted"}`

---

## 14. Подать заявку (студент → рекрутер)

**URL:** `http://localhost:8080/api/applications`  
**Метод:** `POST`  
**Роль:** student  
**Заголовки:** `Authorization: Bearer <token>`, `Content-Type: application/json`  
**Тело запроса (JSON):**

```json
{
  "recruiter_id": "660e8400-e29b-41d4-a716-446655440001",
  "vacancy_id": "550e8400-e29b-41d4-a716-446655440000",
  "invitation_id": "inv-uuid-...",
  "cover_letter": "Хочу присоединиться к вашей команде"
}
```

| Поле          | Тип   | Обязательно | Описание                |
|---------------|-------|-------------|-------------------------|
| recruiter_id  | string| да          | UUID рекрутера          |
| vacancy_id    | string| да          | UUID вакансии           |
| invitation_id | string| нет         | UUID приглашения        |
| cover_letter  | string| нет         | Сопроводительное письмо |

**Пример ответа (200):**
```json
{
  "id": "app-uuid-...",
  "student_id": "550e8400-...",
  "recruiter_id": "660e8400-...",
  "vacancy_id": "550e8400-e29b-41d4-a716-446655440000",
  "vacancy_title": "Junior Go developer",
  "recruiter_company_name": "Uniintern",
  "cover_letter": "Хочу присоединиться к вашей команде",
  "status": "pending",
  "created_at": "2025-02-23T12:00:00Z"
}
```

---

## 15. Список заявок

**URL:** `http://localhost:8080/api/applications`  
**Метод:** `GET`  
**Роль:** student или recruiter  
**Заголовки:** `Authorization: Bearer <token>`  
**Тело:** нет  

**Пример ответа (200):** массив объектов:
```json
[
  {
    "id": "app-uuid-...",
    "student_id": "550e8400-...",
    "recruiter_id": "660e8400-...",
    "vacancy_id": "550e8400-e29b-41d4-a716-446655440000",
    "vacancy_title": "Junior Go developer",
    "recruiter_company_name": "Uniintern",
    "cover_letter": "Текст письма",
    "status": "pending",
    "created_at": "2025-02-23T12:00:00Z"
  }
]
```

---

## 16. Изменить статус заявки (рекрутер)

**URL:** `http://localhost:8080/api/applications?id=<uuid>`  
**Метод:** `PATCH`  
**Роль:** recruiter  
**Заголовки:** `Authorization: Bearer <token>`, `Content-Type: application/json`  
**Query-параметры:** `id` — UUID заявки (из GET /api/applications)  
**Тело запроса (JSON):**

```json
{
  "status": "viewed"
}
```

или `"accepted"` или `"rejected"`.

| Поле   | Тип   | Обязательно | Описание                              |
|--------|-------|-------------|----------------------------------------|
| status | string| да          | `"viewed"` \| `"accepted"` \| `"rejected"` |

**Пример запроса:** `PATCH http://localhost:8080/api/applications?id=app-uuid-...`  
**Пример ответа (200):** `{"status": "accepted"}`

---

## 17. Поиск пользователей

**URL:** `http://localhost:8080/api/search/users?role=<role>&email=<префикс>`  
**Метод:** `GET`  
**Роль:** admin или recruiter (рекрутер — только по студентам)  
**Заголовки:** `Authorization: Bearer <token>`  
**Query-параметры:**

| Параметр | Тип   | Обязательно | Описание                          |
|----------|-------|-------------|-----------------------------------|
| role     | string| нет         | `"student"` \| `"recruiter"`       |
| email    | string| нет         | Префикс email (поиск LIKE prefix%) |

**Пример запроса:** `GET http://localhost:8080/api/search/users?role=student&email=stu`

**Пример ответа (200):** массив пользователей (без паролей):
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "student@test.com",
    "role": "student"
  }
]
```

---

## Краткая сводка по запросам

| №  | Метод | URL | Тело / Query |
|----|-------|-----|----------------|
| 1  | GET   | `/health` | — |
| 2  | POST  | `/api/auth/register` | JSON: email, password, role |
| 3  | POST  | `/api/auth/login` | JSON: email, password |
| 4  | GET   | `/api/me` | Header: Authorization |
| 5  | PUT/PATCH | `/api/me/profile` | JSON: full_name?, phone?, bio?, skills?, education?, experience_years?, location?, availability? |
| 6  | PUT/PATCH | `/api/me/recruiter` | JSON: company_name?, full_name?, phone? |
| 7  | GET   | `/api/users?id=<uuid>` | Query: id |
| 8  | POST  | `/api/files/resume` | multipart: resume или file (PDF) |
| 9  | POST  | `/api/files/logo` | multipart: logo |
| 10 | GET   | `/api/files/url?key=<key>` | Query: key |
| 11 | POST  | `/api/invitations` | JSON: student_id, message? |
| 12 | GET   | `/api/invitations` | — |
| 13 | PATCH | `/api/invitations?id=<uuid>` | JSON: status (accepted\|declined) |
| 14 | POST  | `/api/applications` | JSON: recruiter_id, vacancy_id, invitation_id?, cover_letter? |
| 15 | GET   | `/api/applications` | — (каждый элемент содержит vacancy_id, vacancy_title, recruiter_company_name) |
| 16 | PATCH | `/api/applications?id=<uuid>` | JSON: status (viewed\|accepted\|rejected) |
| 17 | GET   | `/api/search/users?role=&email=` | Query: role?, email? |
| 18 | POST  | `/api/vacancies` | JSON: title, description?, required_skills?, location?, employment_type?, min_experience_years? |
| 19 | GET   | `/api/vacancies` | — (список) или ?id=<uuid> (одна вакансия) |
| 20 | GET   | `/api/vacancies/mine` | — (вакансии текущего рекрутера) |
| 21 | PUT/PATCH | `/api/vacancies?id=<uuid>` | JSON: title?, description?, required_skills?, location?, employment_type?, min_experience_years? |
| 22 | DELETE | `/api/vacancies?id=<uuid>` | — |
| 23 | GET   | `/api/match/vacancy?id=<vacancy_uuid>` | — (кандидаты по вакансии, recruiter) |
| 24 | GET   | `/api/match/recommendations` | — (рекомендованные вакансии, student) |
| 25 | GET   | `/api/search/students?skills=&experience_min=&location=&education=` | Query: skills?, experience_min?, location?, education? (recruiter/admin) |

Ошибки от API приходят в JSON: `{"error": "текст ошибки"}`.
