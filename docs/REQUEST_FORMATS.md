# Request formats: JSON and other

Base URL: `http://localhost:8080`  
Protected routes need: `Authorization: Bearer <token>`

---

## No body

| Method | URL | Headers |
|--------|-----|---------|
| GET | `/health` | — |
| GET | `/api/me` | Authorization |
| GET | `/api/users?id=<uuid>` | Authorization |
| GET | `/api/invitations` | Authorization |
| GET | `/api/applications` | Authorization |
| GET | `/api/files/url?key=<object_key>` | Authorization |
| GET | `/api/search/users?role=student&email=test` | Authorization |

---

## JSON format

**Header:** `Content-Type: application/json`

### POST /api/auth/register
```json
{
  "email": "user@example.com",
  "password": "YourPassword123",
  "role": "student"
}
```
`role`: `"student"` | `"recruiter"` | `"admin"`

---

### POST /api/auth/login
```json
{
  "email": "user@example.com",
  "password": "YourPassword123"
}
```

---

### PUT or PATCH /api/me/profile (student)
```json
{
  "full_name": "Ivan Ivanov",
  "phone": "+79001234567",
  "bio": "Student, looking for internship"
}
```

---

### PUT or PATCH /api/me/recruiter
```json
{
  "company_name": "Company LLC",
  "full_name": "Petr Petrov",
  "phone": "+79007654321"
}
```

---

### POST /api/invitations (recruiter)
```json
{
  "student_id": "550e8400-e29b-41d4-a716-446655440000",
  "message": "We invite you to an internship."
}
```

---

### PATCH /api/invitations?id=<invitation_uuid> (student)
```json
{
  "status": "accepted"
}
```
`status`: `"accepted"` | `"declined"`

---

### POST /api/applications (student)
```json
{
  "recruiter_id": "660e8400-e29b-41d4-a716-446655440001",
  "invitation_id": "770e8400-e29b-41d4-a716-446655440002",
  "cover_letter": "I am interested in this position."
}
```
`invitation_id` is optional.

---

### PATCH /api/applications?id=<application_uuid> (recruiter)
```json
{
  "status": "accepted"
}
```
`status`: `"viewed"` | `"accepted"` | `"rejected"`

---

## Multipart form format (file upload)

**Header:** `Content-Type: multipart/form-data` (set automatically by client)

### POST /api/files/resume (student)

| Key   | Type | Value        |
|-------|------|--------------|
| resume **or** file | File | your PDF file |

- Only **PDF**.
- Max size **5 MB**.
- Field name must be exactly `resume` or `file` (lowercase).

**Example (curl):**
```bash
curl -X POST http://localhost:8080/api/files/resume \
  -H "Authorization: Bearer <token>" \
  -F "file=@/path/to/resume.pdf"
```

---

### POST /api/files/logo (recruiter)

| Key  | Type | Value        |
|------|------|--------------|
| logo | File | image file   |

- Allowed: **PNG**, **JPG**, **JPEG**, **WEBP**.
- Max size **2 MB**.

**Example (curl):**
```bash
curl -X POST http://localhost:8080/api/files/logo \
  -H "Authorization: Bearer <token>" \
  -F "logo=@/path/to/logo.png"
```

---

## Summary table

| Endpoint | Content-Type | Body format |
|----------|--------------|-------------|
| POST /api/auth/register | application/json | JSON |
| POST /api/auth/login | application/json | JSON |
| PUT,PATCH /api/me/profile | application/json | JSON |
| PUT,PATCH /api/me/recruiter | application/json | JSON |
| POST /api/invitations | application/json | JSON |
| PATCH /api/invitations?id= | application/json | JSON |
| POST /api/applications | application/json | JSON |
| PATCH /api/applications?id= | application/json | JSON |
| POST /api/files/resume | multipart/form-data | form: resume or file (PDF) |
| POST /api/files/logo | multipart/form-data | form: logo (image) |
| GET /health, /api/me, /api/users, etc. | — | no body |
