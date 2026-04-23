# Все запросы API (только запросы)

Base URL: `http://localhost:8080`  
Защищённые: заголовок `Authorization: Bearer <token>`

---

## Без токена

```
GET  /health
POST /api/auth/register   Body: { "email", "password", "role" }   role: student | recruiter | admin
POST /api/auth/login      Body: { "email", "password" }
```

---

## С токеном

```
GET  /api/me
PUT  /api/me/profile      Body: { "full_name", "phone", "bio" }                    (student)
PATCH /api/me/profile     Body: { "full_name"?, "phone"?, "bio"? }
PUT  /api/me/recruiter    Body: { "company_name", "full_name", "phone" }           (recruiter)
PATCH /api/me/recruiter   Body: { "company_name"?, "full_name"?, "phone"? }

GET  /api/users?id=<uuid>

POST /api/files/resume    multipart: поле "resume" или "file" (PDF, до 5 МБ)         (student)
POST /api/files/logo      multipart: поле "logo" (PNG/JPG/WEBP, до 2 МБ)            (recruiter)
GET  /api/files/url?key=<object_key>

POST   /api/invitations   Body: { "student_id", "message" }                         (recruiter)
GET    /api/invitations
PATCH  /api/invitations?id=<uuid>   Body: { "status": "accepted" | "declined" }     (student)

POST   /api/applications  Body: { "recruiter_id", "invitation_id"?, "cover_letter" } (student)
GET    /api/applications
PATCH  /api/applications?id=<uuid>   Body: { "status": "viewed" | "accepted" | "rejected" } (recruiter)

GET  /api/search/users?role=student|recruiter&email=<префикс>   (admin, recruiter — только role=student)
```
