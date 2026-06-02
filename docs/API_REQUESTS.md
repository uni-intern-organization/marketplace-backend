# API: все запросы и тестовые примеры

Base URL: `http://localhost:8080`

Для защищённых запросов добавьте заголовок:  
`Authorization: Bearer <ваш_jwt_токен>`

---

## 1. Health (без авторизации)

**Назначение:** проверка, что сервер запущен.

| Метод | URL | Тело |
|-------|-----|------|
| GET | `/health` | — |

**Пример запроса (curl):**
```bash
curl http://localhost:8080/health
```
**Ответ:** `200 OK`, тело: `ok`

---

## 2. Регистрация

**Назначение:** создание нового пользователя. Роль: `student`, `recruiter` или `admin`. Повторная регистрация с тем же email вернёт 409.

| Метод | URL | Тело |
|-------|-----|------|
| POST | `/api/auth/register` | JSON |

**Пример JSON:**
```json
{
  "email": "student@test.com",
  "password": "SecurePass123",
  "role": "student"
}
```

**Другие роли:**
```json
{"email": "recruiter@company.com", "password": "RecruiterPass1", "role": "recruiter"}
```
```json
{"email": "admin@site.com", "password": "AdminPass1", "role": "admin"}
```

**Ответ (200):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "student@test.com",
    "role": "student"
  }
}
```

---

## 3. Вход (логин)

**Назначение:** получение JWT по email и паролю.

| Метод | URL | Тело |
|-------|-----|------|
| POST | `/api/auth/login` | JSON |

**Пример JSON:**
```json
{
  "email": "student@test.com",
  "password": "SecurePass123"
}
```

**Ответ (200):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "student@test.com",
    "role": "student"
  }
}
```
Токен из поля `token` подставляйте в заголовок `Authorization: Bearer <token>` для всех запросов ниже.

---

## 4. Текущий пользователь и профиль (GET /api/me)

**Назначение:** данные пользователя из токена и его профиль (студент/рекрутер). Если профиля ещё нет — вернётся пустой объект профиля.

| Метод | URL | Роль | Тело |
|-------|-----|------|------|
| GET | `/api/me` | student, recruiter, admin | — |

**Заголовок:** `Authorization: Bearer <token>`

**Ответ студента (200):**
```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "student@test.com",
  "role": "student",
  "profile": {
    "full_name": "Иван Иванов",
    "phone": "+79001234567",
    "bio": "Студент 3 курса, ищу стажировку",
    "resume_url": "resumes/550e8400-.../file.pdf"
  }
}
```
Если профиль не заполнялся — поля в `profile` пустые или `null`.

**Ответ рекрутера (200):**
```json
{
  "user_id": "660e8400-e29b-41d4-a716-446655440001",
  "email": "recruiter@company.com",
  "role": "recruiter",
  "profile": {
    "company_name": "ООО Рога и Копыта",
    "full_name": "Пётр Петров",
    "phone": "+79007654321",
    "logo_url": "logos/660e8400-.../logo.png"
  }
}
```

---

## 5. Обновить профиль студента

**Назначение:** создать или обновить профиль студента (ФИО, телефон, био). Данные шифруются (AES-256).

| Метод | URL | Роль | Тело |
|-------|-----|------|------|
| PUT или PATCH | `/api/me/profile` | student | JSON |

**Пример JSON:**
```json
{
  "full_name": "Иван Иванов",
  "phone": "+79001234567",
  "bio": "Студент 3 курса, backend на Go, ищу стажировку"
}
```
Можно передавать не все поля — обновятся только переданные.

**Ответ (200):**
```json
{"status": "ok"}
```

---

## 6. Обновить профиль рекрутера

**Назначение:** создать или обновить профиль компании/рекрутера (название компании, ФИО, телефон). Данные шифруются.

| Метод | URL | Роль | Тело |
|-------|-----|------|------|
| PUT или PATCH | `/api/me/recruiter` | recruiter | JSON |

**Пример JSON:**
```json
{
  "company_name": "ООО Рога и Копыта",
  "full_name": "Пётр Петров",
  "phone": "+79007654321"
}
```

**Ответ (200):**
```json
{"status": "ok"}
```

---

## 7. Получить пользователя по ID

**Назначение:** данные пользователя по UUID. Студент/рекрутер могут запросить только себя; admin — любого.

| Метод | URL | Роль | Тело |
|-------|-----|------|------|
| GET | `/api/users?id=<uuid>` | student, recruiter, admin | — |

**Пример запроса:**  
`GET /api/users?id=550e8400-e29b-41d4-a716-446655440000`

**Ответ (200):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "student@test.com",
  "role": "student"
}
```

---

## 8. Загрузить резюме (PDF)

**Назначение:** загрузка PDF-резюме студента в S3/MinIO. В профиле сохраняется ключ объекта.

| Метод | URL | Роль | Тело |
|-------|-----|------|------|
| POST | `/api/files/resume` | student | multipart/form-data |

**Поле формы:** `resume` **или** `file` — файл (только PDF). До 5 МБ.

**Пример в Postman:** Body → **form-data** → Key: `resume` или `file`, тип **File** → Select Files → выбрать PDF.

**Пример (curl) с полем resume:**
```bash
curl -X POST http://localhost:8080/api/files/resume \
  -H "Authorization: Bearer <token>" \
  -F "resume=@/path/to/resume.pdf"
```

**Пример (curl) с полем file:**
```bash
curl -X POST http://localhost:8080/api/files/resume \
  -H "Authorization: Bearer <token>" \
  -F "file=@C:/Users/Admin/Desktop/resume.pdf"
```

**Ответ (200):**
```json
{"object_key": "resumes/550e8400-e29b-.../abc123.pdf"}
```

---

## 9. Загрузить логотип компании

**Назначение:** загрузка логотипа компании (PNG/JPG/WEBP) в S3/MinIO.

| Метод | URL | Роль | Тело |
|-------|-----|------|------|
| POST | `/api/files/logo` | recruiter | multipart/form-data |

**Поле формы:** `logo` — файл (PNG, JPG, JPEG, WEBP).

**Пример (curl):**
```bash
curl -X POST http://localhost:8080/api/files/logo \
  -H "Authorization: Bearer <token>" \
  -F "logo=@/path/to/logo.png"
```

**Ответ (200):**
```json
{"object_key": "logos/660e8400-.../xyz789.png"}
```

---

## 10. Получить ссылку на скачивание файла

**Назначение:** временная (presigned) ссылка для скачивания файла по ключу из S3/MinIO.

| Метод | URL | Роль | Тело |
|-------|-----|------|------|
| GET | `/api/files/url?key=<object_key>` | all | — |

**Пример запроса:**  
`GET /api/files/url?key=resumes/550e8400-.../abc123.pdf`

**Ответ (200):**
```json
{"url": "http://minio:9000/marketplace/resumes/...?X-Amz-..."}
```

---

## 11. Создать приглашение (рекрутер → студент)

**Назначение:** рекрутер отправляет приглашение студенту (по его user_id). Сообщение шифруется.

| Метод | URL | Роль | Тело |
|-------|-----|------|------|
| POST | `/api/invitations` | recruiter | JSON |

**Пример JSON:**
```json
{
  "student_id": "550e8400-e29b-41d4-a716-446655440000",
  "message": "Приглашаем вас на стажировку в отдел разработки. Свяжитесь с нами для собеседования."
}
```
`student_id` — UUID студента (можно получить через поиск или из ответа регистрации).

**Ответ (200):**
```json
{
  "id": "770e8400-e29b-41d4-a716-446655440002",
  "recruiter_id": "660e8400-e29b-41d4-a716-446655440001",
  "student_id": "550e8400-e29b-41d4-a716-446655440000",
  "message": "Приглашаем вас на стажировку...",
  "status": "pending",
  "created_at": "2026-02-23T19:00:00Z"
}
```

---

## 12. Список своих приглашений

**Назначение:** студент видит приглашения себе; рекрутер — отправленные им приглашения.

| Метод | URL | Роль | Тело |
|-------|-----|------|------|
| GET | `/api/invitations` | student, recruiter | — |

**Ответ (200) — массив:**
```json
[
  {
    "id": "770e8400-e29b-41d4-a716-446655440002",
    "recruiter_id": "660e8400-...",
    "student_id": "550e8400-...",
    "message": "Приглашаем вас на стажировку...",
    "status": "pending",
    "created_at": "2026-02-23T19:00:00Z"
  }
]
```

---

## 13. Принять или отклонить приглашение

**Назначение:** студент меняет статус приглашения на `accepted` или `declined`.

| Метод | URL | Роль | Тело |
|-------|-----|------|------|
| PATCH | `/api/invitations?id=<invitation_uuid>` | student | JSON |

**Пример запроса:**  
`PATCH /api/invitations?id=770e8400-e29b-41d4-a716-446655440002`

**Пример JSON (принять):**
```json
{"status": "accepted"}
```

**Пример JSON (отклонить):**
```json
{"status": "declined"}
```

**Ответ (200):**
```json
{"status": "accepted"}
```

---

## 14. Подать заявку (студент → рекрутер)

**Назначение:** студент отправляет заявку рекрутеру. Можно привязать к приглашению (`invitation_id`) или отправить без него. Сопроводительное письмо шифруется.

| Метод | URL | Роль | Тело |
|-------|-----|------|------|
| POST | `/api/applications` | student | JSON |

**Пример с приглашением:**
```json
{
  "recruiter_id": "660e8400-e29b-41d4-a716-446655440001",
  "invitation_id": "770e8400-e29b-41d4-a716-446655440002",
  "cover_letter": "Готов пройти собеседование. Интересуюсь backend-разработкой на Go."
}
```

**Пример без приглашения:**
```json
{
  "recruiter_id": "660e8400-e29b-41d4-a716-446655440001",
  "cover_letter": "Хотел бы откликнуться на вакансию стажёра."
}
```

**Ответ (200):**
```json
{
  "id": "880e8400-e29b-41d4-a716-446655440003",
  "student_id": "550e8400-...",
  "recruiter_id": "660e8400-...",
  "cover_letter": "Готов пройти собеседование...",
  "status": "submitted",
  "created_at": "2026-02-23T19:30:00Z"
}
```

---

## 15. Список своих заявок

**Назначение:** студент видит свои заявки; рекрутер — заявки, пришедшие к нему.

| Метод | URL | Роль | Тело |
|-------|-----|------|------|
| GET | `/api/applications` | student, recruiter | — |

**Ответ (200) — массив:**
```json
[
  {
    "id": "880e8400-e29b-41d4-a716-446655440003",
    "student_id": "550e8400-...",
    "recruiter_id": "660e8400-...",
    "cover_letter": "Готов пройти собеседование...",
    "status": "submitted",
    "created_at": "2026-02-23T19:30:00Z"
  }
]
```

---

## 16. Изменить статус заявки (рекрутер)

**Назначение:** рекрутер помечает заявку как просмотренную, принятую или отклонённую.

| Метод | URL | Роль | Тело |
|-------|-----|------|------|
| PATCH | `/api/applications?id=<application_uuid>` | recruiter | JSON |

**Пример запроса:**  
`PATCH /api/applications?id=880e8400-e29b-41d4-a716-446655440003`

**Примеры JSON:**
```json
{"status": "viewed"}
```
```json
{"status": "accepted"}
```
```json
{"status": "rejected"}
```

**Ответ (200):**
```json
{"status": "accepted"}
```

---

## 17. Поиск пользователей

**Назначение:** admin — поиск по роли и префиксу email; recruiter — только студенты (по префиксу email). Результат — до 50 пользователей.

| Метод | URL | Роль | Тело |
|-------|-----|------|------|
| GET | `/api/search/users?role=student&email=<prefix>` | admin, recruiter | — |

**Параметры (query):**
- `role` — опционально: `student` или `recruiter` (для recruiter доступен только `student`).
- `email` — опционально: префикс email (например `stu` для `student@test.com`).

**Примеры запросов:**  
- Все студенты: `GET /api/search/users?role=student`  
- Студенты с email на "test": `GET /api/search/users?role=student&email=test`  
- Рекрутер ищет студентов: `GET /api/search/users?role=student&email=stu`

**Ответ (200) — массив:**
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

## Биллинг (mock, только recruiter)

**Назначение:** демонстрация бизнес-модели для компаний. Реальные платежи не выполняются — подписка и продвижение активируются сразу.

Студенты **не** используют эти эндпоинты и **не** имеют тарифных ограничений.

### Коды ошибок (403)

| code | Когда |
|------|--------|
| `subscription_required` | Поиск студентов, приглашения, matching, аналитика без Pro |
| `plan_limit_reached` | Вторая вакансия на тарифе Free |

Тело ошибки:
```json
{"error":"student search requires Pro subscription","code":"subscription_required"}
```

### GET `/api/billing/plans`

**Роль:** recruiter, admin. **Ответ:** список тарифов `free` / `pro` и блок `promotion` (продвижение вакансии).

### GET `/api/billing/me`

**Роль:** recruiter, admin. **Ответ:**
```json
{
  "plan": "free",
  "plan_expires_at": null,
  "vacancy_count": 1,
  "max_vacancies": 1,
  "is_pro": false,
  "features": ["view_applications", "post_one_vacancy"]
}
```

### POST `/api/billing/subscribe`

**Роль:** recruiter. **Тело:**
```json
{ "plan": "pro" }
```
**Эффект:** `plan=pro`, `plan_expires_at = now + 30 days`, запись в `billing_events`.

### POST `/api/billing/promote-vacancy`

**Роль:** recruiter. **Тело:**
```json
{ "vacancy_id": "<uuid>" }
```
**Эффект:** `is_featured=true`, `featured_until = now + 7 days` для своей вакансии.

### GET `/api/billing/analytics`

**Роль:** recruiter (только Pro). **Ответ:** счётчики вакансий, откликов по статусам, приглашений.

### GET `/api/me` (дополнение для recruiter)

В ответ добавлено поле `billing`:
```json
{
  "user_id": "...",
  "email": "recruiter@company.com",
  "role": "recruiter",
  "profile": { "company_name": "..." },
  "billing": {
    "plan": "free",
    "is_pro": false,
    "features": ["view_applications", "post_one_vacancy"]
  }
}
```

### Публичный каталог вакансий

`GET /api/vacancies` сортирует вакансии: сначала активно продвинутые (`is_featured` и `featured_until > now()`), затем по дате создания.

---

## Сводная таблица (быстрый тест)

| # | Метод | URL | Роль | Пример тела |
|---|--------|-----|------|-------------|
| 1 | GET | `/health` | — | — |
| 2 | POST | `/api/auth/register` | — | `{"email":"u@t.com","password":"123","role":"student"}` |
| 3 | POST | `/api/auth/login` | — | `{"email":"u@t.com","password":"123"}` |
| 4 | GET | `/api/me` | all | — |
| 5 | PUT | `/api/me/profile` | student | `{"full_name":"Иван","phone":"+7...","bio":"..."}` |
| 6 | PUT | `/api/me/recruiter` | recruiter | `{"company_name":"ООО","full_name":"Петр","phone":"+7..."}` |
| 7 | GET | `/api/users?id=<uuid>` | all | — |
| 8 | POST | `/api/files/resume` | student | form: `resume` (file) |
| 9 | POST | `/api/files/logo` | recruiter | form: `logo` (file) |
| 10 | GET | `/api/files/url?key=<key>` | all | — |
| 11 | POST | `/api/invitations` | recruiter | `{"student_id":"<uuid>","message":"Текст"}` |
| 12 | GET | `/api/invitations` | student, recruiter | — |
| 13 | PATCH | `/api/invitations?id=<uuid>` | student | `{"status":"accepted"}` или `"declined"` |
| 14 | POST | `/api/applications` | student | `{"recruiter_id":"<uuid>","cover_letter":"..."}` |
| 15 | GET | `/api/applications` | student, recruiter | — |
| 16 | PATCH | `/api/applications?id=<uuid>` | recruiter | `{"status":"viewed"}` / `"accepted"` / `"rejected"` |
| 17 | GET | `/api/search/users?role=student&email=...` | admin, recruiter | — |
| 18 | GET | `/api/billing/plans` | recruiter, admin | — |
| 19 | GET | `/api/billing/me` | recruiter, admin | — |
| 20 | POST | `/api/billing/subscribe` | recruiter | `{"plan":"pro"}` |
| 21 | POST | `/api/billing/promote-vacancy` | recruiter | `{"vacancy_id":"<uuid>"}` |
| 22 | GET | `/api/billing/analytics` | recruiter (Pro) | — |
| 23 | POST | `/api/billing/publish-vacancy` | recruiter | `{"vacancy_id":"<uuid>","listing_tier":"premium"}` |
| 24 | POST | `/api/vacancies/renew?id=<uuid>` | recruiter | `{"listing_tier":"basic"}` |
| 25 | GET | `/api/students/{id}/portfolio` | student/recruiter/admin | — |
| 26 | GET | `/api/freelance/tasks` | public / mine | `?mine=true` |
| 27 | POST | `/api/freelance/tasks` | recruiter | task JSON |
| 28 | POST | `/api/freelance/proposals?task_id=` | student | `{"message":"..."}` |
| 29 | POST | `/api/freelance/tasks/complete?task_id=` | recruiter | — |
| 30 | GET | `/api/hackathons` | public | `?id=` / `?mine=true` |
| 31 | POST | `/api/hackathons` | recruiter | hackathon JSON |
| 32 | POST | `/api/hackathons/publish?id=` | recruiter | — |
| 33 | GET | `/api/match/recommendations` | student | unified: vacancies + freelance + hackathons |

Все запросы, кроме 1–3, публичного `GET /api/vacancies`, `GET /api/freelance/tasks`, `GET /api/hackathons` — требуют заголовок: `Authorization: Bearer <token>`.

См. также раздел «Бизнес-модель» в [docs/BUSINESS_MODEL.md](../../docs/BUSINESS_MODEL.md).
