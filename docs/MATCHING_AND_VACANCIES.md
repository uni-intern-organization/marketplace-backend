# Job Vacancies, Skill-Based Profiles & Matching (Week 3)

This document describes what was added to address the backend review: job posting, skill-based student profiles, matching engine, and recommendations.

---

## 1. Job vacancy system

**Recruiters** can create and manage internship vacancies.

- **POST /api/vacancies** (recruiter)  
  Body: `title` (required), `description`, `required_skills` (comma-separated), `location`, `employment_type` (e.g. remote, hybrid, onsite), `min_experience_years`.

- **GET /api/vacancies**  
  Returns all vacancies (with optional query filters).  
  **GET /api/vacancies?id=&lt;uuid&gt;** returns one vacancy.

- **GET /api/vacancies/mine** (recruiter)  
  Returns vacancies created by the current recruiter.

- **PUT /api/vacancies?id=&lt;uuid&gt;** or **PATCH** (recruiter)  
  Update a vacancy (same body shape as create).

- **DELETE /api/vacancies?id=&lt;uuid&gt;** (recruiter)  
  Delete a vacancy.

**List filters (query params):** `skills`, `location`, `employment_type`, `min_experience_years`, `limit`.

---

## 2. Skill-based student profiles

**Students** can fill in data used for matching and search.

- **PUT/PATCH /api/me/profile** (student)  
  In addition to `full_name`, `phone`, `bio`, the body now supports:
  - **skills** — comma-separated (e.g. `"Go, React, SQL"`)
  - **education** — free text
  - **experience_years** — number
  - **location** — free text
  - **availability** — e.g. `remote`, `hybrid`, `onsite`

- **GET /api/me**  
  Response for students now includes `skills`, `education`, `experience_years`, `location`, `availability` in `profile`.

These fields are stored in plain text so the backend can filter and compute match scores. Sensitive data (e.g. full name, phone) remains encrypted.

---

## 3. Matching engine

- **GET /api/match/vacancy?id=&lt;vacancy_id&gt;** (recruiter)  
  Returns **candidates** for that vacancy: list of students with profiles, sorted by **match_score**.  
  Score is based on: overlapping skills, location match, availability/employment type match, and experience vs `min_experience_years`.

- **GET /api/match/recommendations** (student)  
  Returns **recommended vacancies** for the current student, sorted by **match_score** using the same logic (skills, location, availability, experience).

Scoring is simple and deterministic: +10 per matching skill, +5 for location match, +5 for availability/employment type match, +5 if student experience ≥ vacancy min, small penalty if below.

---

## 4. Skill-based search (students)

- **GET /api/search/students** (recruiter or admin)  
  Returns students (with profile data) filtered by:
  - **skills** — comma-separated; student must have at least one matching skill
  - **experience_min** — minimum experience years
  - **location** — substring match in student location
  - **education** — substring match in student education

---

## 5. Database changes

Migration **000002_matching_and_vacancies.up.sql** adds:

- **student_profiles:** `skills`, `education`, `experience_years`, `location`, `availability`
- **vacancies:** `required_skills`, `location`, `employment_type`, `min_experience_years`

Existing rows get default values (empty string or 0). No data loss.

---

## Summary

| Feature                         | Status   | Endpoints / behaviour |
|---------------------------------|----------|------------------------|
| Job vacancy posting              | Done     | POST/GET/PUT/PATCH/DELETE /api/vacancies, GET /api/vacancies/mine |
| Skill-based student profiles     | Done     | Extended PUT/PATCH /api/me/profile, GET /api/me |
| Matching: candidates per vacancy | Done     | GET /api/match/vacancy?id= (recruiter) |
| Recommendations for students    | Done     | GET /api/match/recommendations (student) |
| Skill-based student search       | Done     | GET /api/search/students (recruiter/admin) |

All of the above are implemented and wired in the API. Frontend can use the structures in **API_REQUEST_STRUCTURES_FOR_FRONTEND.md** (including the new rows in the summary table).
