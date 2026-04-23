# Каждый запрос: как отправить, какой токен, какие заголовки

Base URL: `http://localhost:8080`  
Токен получают из ответа **POST /api/auth/login** или **POST /api/auth/register** (поле `token`).  
Для запросов «Токен: да» обязательно добавлять заголовок: **`Authorization: Bearer <ваш_токен>`**.

---

## 1. Health

| | |
|--|--|
| **Что делает** | Проверка, что сервер доступен |
| **URL** | `http://localhost:8080/health` |
| **Метод** | `GET` |
| **Токен** | **Не нужен** |
| **Заголовки** | Ничего не отправлять |
| **Тело** | Нет |
| **Query** | Нет |

**Как отправить:**  
`GET http://localhost:8080/health` — без заголовков.

---

## 2. Регистрация

| | |
|--|--|
| **Что делает** | Создание нового пользователя |
| **URL** | `http://localhost:8080/api/auth/register` |
| **Метод** | `POST` |
| **Токен** | **Не нужен** |
| **Заголовки** | `Content-Type: application/json` |
| **Тело** | JSON (см. ниже) |
| **Query** | Нет |

**Тело (JSON):**
```json
{
  "email": "user@example.com",
  "password": "password123",
  "role": "student"
}
```
`role` — строго одно из: `"student"`, `"recruiter"`, `"admin"`.

**Как отправить:**  
Заголовок: `Content-Type: application/json`.  
Body: указанный JSON. Токен не передавать.

---

## 3. Вход (логин)

| | |
|--|--|
| **Что делает** | Получение JWT по email и паролю |
| **URL** | `http://localhost:8080/api/auth/login` |
| **Метод** | `POST` |
| **Токен** | **Не нужен** |
| **Заголовки** | `Content-Type: application/json` |
| **Тело** | JSON (см. ниже) |
| **Query** | Нет |

**Тело (JSON):**
```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

**Как отправить:**  
Заголовок: `Content-Type: application/json`.  
Body: указанный JSON. Токен не передавать.  
В ответе взять `token` и использовать его во всех запросах ниже, где указано «Токен: да».

---

## 4. Текущий пользователь и профиль

| | |
|--|--|
| **Что делает** | Данные текущего пользователя и его профиль |
| **URL** | `http://localhost:8080/api/me` |
| **Метод** | `GET` |
| **Токен** | **Да** — любая роль (student, recruiter, admin) |
| **Заголовки** | `Authorization: Bearer <token>` |
| **Тело** | Нет |
| **Query** | Нет |

**Как отправить:**  
Заголовок: `Authorization: Bearer <ваш_токен>`. Body и query не нужны.

---

## 5. Обновить профиль студента

| | |
|--|--|
| **Что делает** | Создание/обновление профиля студента (ФИО, контакты, навыки и т.д.) |
| **URL** | `http://localhost:8080/api/me/profile` |
| **Метод** | `PUT` или `PATCH` |
| **Токен** | **Да** — роль **student** |
| **Заголовки** | `Authorization: Bearer <token>`, `Content-Type: application/json` |
| **Тело** | JSON (см. ниже) |
| **Query** | Нет |

**Тело (JSON):**
```json
{
  "full_name": "Иван Иванов",
  "phone": "+79001234567",
  "bio": "О себе",
  "skills": "Go, React, SQL",
  "education": "CS",
  "experience_years": 1,
  "location": "Moscow",
  "availability": "remote"
}
```
Поля необязательные, можно отправлять только нужные.

**Как отправить:**  
Токен в заголовке: `Authorization: Bearer <token>`. Роль должна быть student.  
Заголовок: `Content-Type: application/json`. Body: JSON выше.

---

## 6. Обновить профиль рекрутера

| | |
|--|--|
| **Что делает** | Создание/обновление профиля рекрутера |
| **URL** | `http://localhost:8080/api/me/recruiter` |
| **Метод** | `PUT` или `PATCH` |
| **Токен** | **Да** — роль **recruiter** |
| **Заголовки** | `Authorization: Bearer <token>`, `Content-Type: application/json` |
| **Тело** | JSON (см. ниже) |
| **Query** | Нет |

**Тело (JSON):**
```json
{
  "company_name": "ООО Компания",
  "full_name": "Пётр Петров",
  "phone": "+79007654321"
}
```

**Как отправить:**  
Токен: `Authorization: Bearer <token>`, роль recruiter.  
Заголовок: `Content-Type: application/json`. Body: JSON выше.

---

## 7. Получить пользователя по ID

| | |
|--|--|
| **Что делает** | Данные пользователя по UUID |
| **URL** | `http://localhost:8080/api/users?id=<uuid>` |
| **Метод** | `GET` |
| **Токен** | **Да** — любая роль; студент/рекрутер могут запросить только себя, admin — любого |
| **Заголовки** | `Authorization: Bearer <token>` |
| **Тело** | Нет |
| **Query** | `id` — UUID пользователя (обязательно) |

**Как отправить:**  
Заголовок: `Authorization: Bearer <token>`.  
URL: `http://localhost:8080/api/users?id=550e8400-e29b-41d4-a716-446655440000`.

---

## 8. Загрузить резюме (PDF)

| | |
|--|--|
| **Что делает** | Загрузка PDF-резюме студента |
| **URL** | `http://localhost:8080/api/files/resume` |
| **Метод** | `POST` |
| **Токен** | **Да** — роль **student** |
| **Заголовки** | `Authorization: Bearer <token>`. Не ставить Content-Type — браузер подставит multipart сам |
| **Тело** | multipart/form-data: поле `resume` или `file` — файл PDF (до 5 МБ) |
| **Query** | Нет |

**Как отправить:**  
Токен: `Authorization: Bearer <token>`, роль student.  
Form-data: ключ `resume` или `file`, значение — файл PDF.

---

## 9. Загрузить логотип компании

| | |
|--|--|
| **Что делает** | Загрузка логотипа компании |
| **URL** | `http://localhost:8080/api/files/logo` |
| **Метод** | `POST` |
| **Токен** | **Да** — роль **recruiter** |
| **Заголовки** | `Authorization: Bearer <token>` |
| **Тело** | multipart/form-data: поле `logo` — файл PNG/JPG/WEBP (до 2 МБ) |
| **Query** | Нет |

**Как отправить:**  
Токен: `Authorization: Bearer <token>`, роль recruiter.  
Form-data: ключ `logo`, значение — файл изображения.

---

## 10. Ссылка на скачивание файла

| | |
|--|--|
| **Что делает** | Временная ссылка для скачивания файла по ключу |
| **URL** | `http://localhost:8080/api/files/url?key=<object_key>` |
| **Метод** | `GET` |
| **Токен** | **Да** — любая роль |
| **Заголовки** | `Authorization: Bearer <token>` |
| **Тело** | Нет |
| **Query** | `key` — object_key из ответа загрузки резюме/логотипа (обязательно) |

**Как отправить:**  
Заголовок: `Authorization: Bearer <token>`.  
URL: `http://localhost:8080/api/files/url?key=resumes/550e8400-.../file.pdf`.

---

## 11. Создать приглашение (рекрутер → студент)

| | |
|--|--|
| **Что делает** | Рекрутер отправляет приглашение студенту |
| **URL** | `http://localhost:8080/api/invitations` |
| **Метод** | `POST` |
| **Токен** | **Да** — роль **recruiter** |
| **Заголовки** | `Authorization: Bearer <token>`, `Content-Type: application/json` |
| **Тело** | JSON (см. ниже) |
| **Query** | Нет |

**Тело (JSON):**
```json
{
  "student_id": "550e8400-e29b-41d4-a716-446655440000",
  "message": "Текст приглашения"
}
```
`student_id` — UUID студента (обязательно).

**Как отправить:**  
Токен: `Authorization: Bearer <token>`, роль recruiter.  
Заголовок: `Content-Type: application/json`. Body: JSON выше.

---

## 12. Список приглашений

| | |
|--|--|
| **Что делает** | Список приглашений (для студента — входящие, для рекрутера — отправленные) |
| **URL** | `http://localhost:8080/api/invitations` |
| **Метод** | `GET` |
| **Токен** | **Да** — роль **student** или **recruiter** |
| **Заголовки** | `Authorization: Bearer <token>` |
| **Тело** | Нет |
| **Query** | Нет |

**Как отправить:**  
Заголовок: `Authorization: Bearer <token>`. Роль student или recruiter.

---

## 13. Принять / отклонить приглашение

| | |
|--|--|
| **Что делает** | Студент меняет статус приглашения |
| **URL** | `http://localhost:8080/api/invitations?id=<uuid>` |
| **Метод** | `PATCH` |
| **Токен** | **Да** — роль **student** |
| **Заголовки** | `Authorization: Bearer <token>`, `Content-Type: application/json` |
| **Тело** | JSON: `{"status": "accepted"}` или `{"status": "declined"}` |
| **Query** | `id` — UUID приглашения (обязательно) |

**Как отправить:**  
Токен: `Authorization: Bearer <token>`, роль student.  
URL: `.../api/invitations?id=<uuid_приглашения>`. Body: `{"status": "accepted"}` или `{"status": "declined"}`.

---

## 14. Подать заявку (студент → рекрутер)

| | |
|--|--|
| **Что делает** | Студент отправляет заявку рекрутеру |
| **URL** | `http://localhost:8080/api/applications` |
| **Метод** | `POST` |
| **Токен** | **Да** — роль **student** |
| **Заголовки** | `Authorization: Bearer <token>`, `Content-Type: application/json` |
| **Тело** | JSON (см. ниже) |
| **Query** | Нет |

**Тело (JSON):**
```json
{
  "recruiter_id": "660e8400-e29b-41d4-a716-446655440001",
  "invitation_id": "inv-uuid-...",
  "cover_letter": "Текст письма"
}
```
`recruiter_id` — обязательно; `invitation_id` и `cover_letter` — по желанию.

**Как отправить:**  
Токен: `Authorization: Bearer <token>`, роль student.  
Заголовок: `Content-Type: application/json`. Body: JSON выше.

---

## 15. Список заявок

| | |
|--|--|
| **Что делает** | Список заявок (студент — свои, рекрутер — входящие) |
| **URL** | `http://localhost:8080/api/applications` |
| **Метод** | `GET` |
| **Токен** | **Да** — роль **student** или **recruiter** |
| **Заголовки** | `Authorization: Bearer <token>` |
| **Тело** | Нет |
| **Query** | Нет |

**Как отправить:**  
Заголовок: `Authorization: Bearer <token>`. Роль student или recruiter.

---

## 16. Изменить статус заявки

| | |
|--|--|
| **Что делает** | Рекрутер меняет статус заявки |
| **URL** | `http://localhost:8080/api/applications?id=<uuid>` |
| **Метод** | `PATCH` |
| **Токен** | **Да** — роль **recruiter** |
| **Заголовки** | `Authorization: Bearer <token>`, `Content-Type: application/json` |
| **Тело** | JSON: `{"status": "viewed"}` или `"accepted"` или `"rejected"` |
| **Query** | `id` — UUID заявки (обязательно) |

**Как отправить:**  
Токен: `Authorization: Bearer <token>`, роль recruiter.  
URL: `.../api/applications?id=<uuid_заявки>`. Body: `{"status": "accepted"}` и т.д.

---

## 17. Поиск пользователей по роли и email

| | |
|--|--|
| **Что делает** | Поиск пользователей (admin — все роли, recruiter — только студенты) |
| **URL** | `http://localhost:8080/api/search/users?role=<role>&email=<префикс>` |
| **Метод** | `GET` |
| **Токен** | **Да** — роль **admin** или **recruiter** |
| **Заголовки** | `Authorization: Bearer <token>` |
| **Тело** | Нет |
| **Query** | `role` — необяз., `"student"` или `"recruiter"`; `email` — необяз., префикс email |

**Как отправить:**  
Токен: `Authorization: Bearer <token>`, роль admin или recruiter.  
URL: `http://localhost:8080/api/search/users?role=student&email=stu`.

---

## 18. Создать вакансию

| | |
|--|--|
| **Что делает** | Рекрутер создаёт вакансию (стажировку) |
| **URL** | `http://localhost:8080/api/vacancies` |
| **Метод** | `POST` |
| **Токен** | **Да** — роль **recruiter** |
| **Заголовки** | `Authorization: Bearer <token>`, `Content-Type: application/json` |
| **Тело** | JSON (см. ниже) |
| **Query** | Нет |

**Тело (JSON):**
```json
{
  "title": "Backend Intern",
  "description": "Go, PostgreSQL",
  "required_skills": "Go, SQL, REST",
  "location": "Remote",
  "employment_type": "remote",
  "min_experience_years": 0
}
```
Обязательно только `title`; остальные поля по желанию.

**Как отправить:**  
Токен: `Authorization: Bearer <token>`, роль recruiter.  
Заголовок: `Content-Type: application/json`. Body: JSON выше.

---

## 19. Список вакансий или одна вакансия

| | |
|--|--|
| **Что делает** | Список вакансий (с фильтрами) или одна вакансия по id |
| **URL** | Список: `http://localhost:8080/api/vacancies`. Одна: `http://localhost:8080/api/vacancies?id=<uuid>` |
| **Метод** | `GET` |
| **Токен** | **Да** — любая роль (student, recruiter, admin) |
| **Заголовки** | `Authorization: Bearer <token>` |
| **Тело** | Нет |
| **Query** | Для списка: `skills`, `location`, `employment_type`, `min_experience_years`, `limit` (все необяз.). Для одной: `id` — UUID вакансии |

**Как отправить:**  
Токен: `Authorization: Bearer <token>`.  
Список: `GET http://localhost:8080/api/vacancies?location=Remote`.  
Одна: `GET http://localhost:8080/api/vacancies?id=<uuid>`.

---

## 20. Мои вакансии (рекрутер)

| | |
|--|--|
| **Что делает** | Вакансии, созданные текущим рекрутером |
| **URL** | `http://localhost:8080/api/vacancies/mine` |
| **Метод** | `GET` |
| **Токен** | **Да** — роль **recruiter** |
| **Заголовки** | `Authorization: Bearer <token>` |
| **Тело** | Нет |
| **Query** | Нет |

**Как отправить:**  
Заголовок: `Authorization: Bearer <token>`, роль recruiter.

---

## 21. Обновить вакансию

| | |
|--|--|
| **Что делает** | Рекрутер редактирует свою вакансию |
| **URL** | `http://localhost:8080/api/vacancies?id=<uuid>` |
| **Метод** | `PUT` или `PATCH` |
| **Токен** | **Да** — роль **recruiter** |
| **Заголовки** | `Authorization: Bearer <token>`, `Content-Type: application/json` |
| **Тело** | JSON: те же поля, что при создании (title, description, required_skills, location, employment_type, min_experience_years) |
| **Query** | `id` — UUID вакансии (обязательно) |

**Как отправить:**  
Токен: `Authorization: Bearer <token>`, роль recruiter.  
URL: `.../api/vacancies?id=<uuid>`. Body: JSON с полями вакансии.

---

## 22. Удалить вакансию

| | |
|--|--|
| **Что делает** | Рекрутер удаляет свою вакансию |
| **URL** | `http://localhost:8080/api/vacancies?id=<uuid>` |
| **Метод** | `DELETE` |
| **Токен** | **Да** — роль **recruiter** |
| **Заголовки** | `Authorization: Bearer <token>` |
| **Тело** | Нет |
| **Query** | `id` — UUID вакансии (обязательно) |

**Как отправить:**  
Токен: `Authorization: Bearer <token>`, роль recruiter.  
Метод: DELETE. URL: `http://localhost:8080/api/vacancies?id=<uuid>`.

---

## 23. Кандидаты по вакансии (matching)

| | |
|--|--|
| **Что делает** | Список студентов, подходящих под вакансию, с полем match_score |
| **URL** | `http://localhost:8080/api/match/vacancy?id=<vacancy_uuid>` |
| **Метод** | `GET` |
| **Токен** | **Да** — роль **recruiter** |
| **Заголовки** | `Authorization: Bearer <token>` |
| **Тело** | Нет |
| **Query** | `id` — UUID вакансии (обязательно) |

**Как отправить:**  
Токен: `Authorization: Bearer <token>`, роль recruiter.  
URL: `http://localhost:8080/api/match/vacancy?id=<uuid_вакансии>`.

---

## 24. Рекомендованные вакансии (студент)

| | |
|--|--|
| **Что делает** | Вакансии, подходящие студенту, с полем match_score |
| **URL** | `http://localhost:8080/api/match/recommendations` |
| **Метод** | `GET` |
| **Токен** | **Да** — роль **student** |
| **Заголовки** | `Authorization: Bearer <token>` |
| **Тело** | Нет |
| **Query** | Нет |

**Как отправить:**  
Заголовок: `Authorization: Bearer <token>`, роль student.

---

## 25. Поиск студентов по навыкам и квалификации

| | |
|--|--|
| **Что делает** | Список студентов с профилями по навыкам, опыту, локации, образованию |
| **URL** | `http://localhost:8080/api/search/students?skills=&experience_min=&location=&education=` |
| **Метод** | `GET` |
| **Токен** | **Да** — роль **recruiter** или **admin** |
| **Заголовки** | `Authorization: Bearer <token>` |
| **Тело** | Нет |
| **Query** | `skills` — через запятую; `experience_min` — число; `location` — подстрока; `education` — подстрока (все необяз.) |

**Как отправить:**  
Токен: `Authorization: Bearer <token>`, роль recruiter или admin.  
URL: `http://localhost:8080/api/search/students?skills=Go,React&experience_min=0`.

---

## Сводка: токен и роль по запросу

| Запрос | Токен | Роль |
|--------|--------|------|
| GET /health | не нужен | — |
| POST /api/auth/register | не нужен | — |
| POST /api/auth/login | не нужен | — |
| GET /api/me | да | student, recruiter, admin |
| PUT,PATCH /api/me/profile | да | student |
| PUT,PATCH /api/me/recruiter | да | recruiter |
| GET /api/users?id= | да | student, recruiter, admin (доступ к себе или любому у admin) |
| POST /api/files/resume | да | student |
| POST /api/files/logo | да | recruiter |
| GET /api/files/url?key= | да | любая |
| POST /api/invitations | да | recruiter |
| GET /api/invitations | да | student, recruiter |
| PATCH /api/invitations?id= | да | student |
| POST /api/applications | да | student |
| GET /api/applications | да | student, recruiter |
| PATCH /api/applications?id= | да | recruiter |
| GET /api/search/users | да | admin, recruiter |
| POST /api/vacancies | да | recruiter |
| GET /api/vacancies, GET /api/vacancies?id= | да | любая |
| GET /api/vacancies/mine | да | recruiter |
| PUT,PATCH /api/vacancies?id= | да | recruiter |
| DELETE /api/vacancies?id= | да | recruiter |
| GET /api/match/vacancy?id= | да | recruiter |
| GET /api/match/recommendations | да | student |
| GET /api/search/students | да | recruiter, admin |

**Ошибки:** при неверном/отсутствующем токене или неподходящей роли сервер вернёт 401/403 и JSON вида `{"error": "..."}`.
