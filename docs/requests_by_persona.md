# Запросы по персонажам (полные URL и тело)

Base: `http://localhost:8081`  
Защищённые запросы: заголовок `Authorization: Bearer <token>` (токен после login).

---

## Без авторизации

### GET health
```
GET http://localhost:8081/health
```
Тело: нет

---

### POST register — Студент 1 (Алексей)
```
POST http://localhost:8081/api/auth/register
Content-Type: application/json
```
```json
{
  "email": "aleksey.student@university.edu",
  "password": "StudentPass2024!",
  "role": "student"
}
```

### POST register — Студент 2 (Мария)
```
POST http://localhost:8081/api/auth/register
Content-Type: application/json
```
```json
{
  "email": "maria.ivanova@mail.ru",
  "password": "MariaSecure99",
  "role": "student"
}
```

### POST register — Рекрутер 1 (Дмитрий)
```
POST http://localhost:8081/api/auth/register
Content-Type: application/json
```
```json
{
  "email": "d.recruiter@techcorp.ru",
  "password": "RecruiterTech1!",
  "role": "recruiter"
}
```

### POST register — Рекрутер 2 (Елена)
```
POST http://localhost:8081/api/auth/register
Content-Type: application/json
```
```json
{
  "email": "elena.hr@startup.io",
  "password": "ElenaHR2024",
  "role": "recruiter"
}
```

### POST register — Админ
```
POST http://localhost:8081/api/auth/register
Content-Type: application/json
```
```json
{
  "email": "admin@marketplace.local",
  "password": "AdminSecure#1",
  "role": "admin"
}
```

---

### POST login — Студент 1
```
POST http://localhost:8081/api/auth/login
Content-Type: application/json
```
```json
{
  "email": "aleksey.student@university.edu",
  "password": "StudentPass2024!"
}
```

### POST login — Студент 2
```
POST http://localhost:8081/api/auth/login
Content-Type: application/json
```
```json
{
  "email": "maria.ivanova@mail.ru",
  "password": "MariaSecure99"
}
```

### POST login — Рекрутер 1
```
POST http://localhost:8081/api/auth/login
Content-Type: application/json
```
```json
{
  "email": "d.recruiter@techcorp.ru",
  "password": "RecruiterTech1!"
}
```

### POST login — Рекрутер 2
```
POST http://localhost:8081/api/auth/login
Content-Type: application/json
```
```json
{
  "email": "elena.hr@startup.io",
  "password": "ElenaHR2024"
}
```

### POST login — Админ
```
POST http://localhost:8081/api/auth/login
Content-Type: application/json
```
```json
{
  "email": "admin@marketplace.local",
  "password": "AdminSecure#1"
}
```

---

## С авторизацией (подставь свой token)

### GET me
```
GET http://localhost:8081/api/me
Authorization: Bearer <token>
```
Тело: нет

---

### PUT me/profile — Студент 1 (Алексей)
```
PUT http://localhost:8081/api/me/profile
Content-Type: application/json
Authorization: Bearer <token>
```
```json
{
  "full_name": "Алексей Студентов",
  "phone": "+7 (912) 345-67-89",
  "bio": "Студент 4 курса МГУ, факультет ВМК. Интересует backend на Go и стажировки в IT."
}
```

### PUT me/profile — Студент 2 (Мария)
```
PUT http://localhost:8081/api/me/profile
Content-Type: application/json
Authorization: Bearer <token>
```
```json
{
  "full_name": "Мария Иванова",
  "phone": "+7 (923) 456-78-90",
  "bio": "Фронтенд-разработка, React. Ищу стажировку на лето 2025."
}
```

---

### PUT me/recruiter — Рекрутер 1 (Дмитрий)
```
PUT http://localhost:8081/api/me/recruiter
Content-Type: application/json
Authorization: Bearer <token>
```
```json
{
  "company_name": "ООО ТехКорп",
  "full_name": "Дмитрий Рекрутеров",
  "phone": "+7 (495) 123-45-67"
}
```

### PUT me/recruiter — Рекрутер 2 (Елена)
```
PUT http://localhost:8081/api/me/recruiter
Content-Type: application/json
Authorization: Bearer <token>
```
```json
{
  "company_name": "Стартап Инновации",
  "full_name": "Елена HR",
  "phone": "+7 (916) 789-01-23"
}
```

---

### GET users
```
GET http://localhost:8081/api/users?id=USER_UUID
Authorization: Bearer <token>
```
Тело: нет (подставь UUID пользователя в query)

---

### POST files/resume (студент)
```
POST http://localhost:8081/api/files/resume
Authorization: Bearer <token>
Content-Type: multipart/form-data
```
Body: form-data, ключ `file` или `resume`, тип File — выбрать PDF.

---

### POST files/logo (рекрутер)
```
POST http://localhost:8081/api/files/logo
Authorization: Bearer <token>
Content-Type: multipart/form-data
```
Body: form-data, ключ `logo`, тип File — выбрать изображение.

---

### GET files/url
```
GET http://localhost:8081/api/files/url?key=OBJECT_KEY
Authorization: Bearer <token>
```
Тело: нет (подставь ключ объекта из ответа загрузки)

---

### POST invitations (рекрутер → студент)
```
POST http://localhost:8081/api/invitations
Content-Type: application/json
Authorization: Bearer <token>
```
```json
{
  "student_id": "UUID_СТУДЕНТА",
  "message": "Здравствуйте! Приглашаем вас на стажировку в отдел разработки. Срок: 3 месяца, возможность удалёнки."
}
```
Подставь реальный UUID студента (из ответа register или GET search/users).

---

### GET invitations
```
GET http://localhost:8081/api/invitations
Authorization: Bearer <token>
```
Тело: нет

---

### PATCH invitations (студент — принять/отклонить)
```
PATCH http://localhost:8081/api/invitations?id=INVITATION_UUID
Content-Type: application/json
Authorization: Bearer <token>
```
```json
{
  "status": "accepted"
}
```
или `"status": "declined"`. Подставь UUID приглашения.

---

### POST applications (студент)
```
POST http://localhost:8081/api/applications
Content-Type: application/json
Authorization: Bearer <token>
```
```json
{
  "recruiter_id": "UUID_РЕКРУТЕРА",
  "cover_letter": "Добрый день! Откликаюсь на приглашение. Готов обсудить детали. С уважением, Алексей."
}
```
Подставь UUID рекрутера. Поле `invitation_id` можно добавить или не указывать.

---

### GET applications
```
GET http://localhost:8081/api/applications
Authorization: Bearer <token>
```
Тело: нет

---

### PATCH applications (рекрутер)
```
PATCH http://localhost:8081/api/applications?id=APPLICATION_UUID
Content-Type: application/json
Authorization: Bearer <token>
```
```json
{
  "status": "accepted"
}
```
или `"status": "viewed"`, `"status": "rejected"`. Подставь UUID заявки.

---

### GET search/users (админ / рекрутер)
```
GET http://localhost:8081/api/search/users?role=student&email=aleksey
Authorization: Bearer <token>
```
Тело: нет. Параметры: `role` (student/recruiter), `email` (префикс).
