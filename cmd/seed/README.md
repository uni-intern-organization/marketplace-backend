# Seed Scripts - Создание тестовых данных

## Описание

Скрипты для создания тестовых студентов с разными специальностями.

### 10 Создаваемых Студентов

| # | Имя | Email | Специальность | Опыт | Город |
|---|---|---|---|---|---|
| 1 | Дмитрий Иванов | dmitry.frontend@example.com | Frontend Developer | 2 года | Almaty |
| 2 | Елена Картузова | elena.backend@example.com | Backend Developer | 3 года | Almaty |
| 3 | Александр Смирнов | alex.fullstack@example.com | Full Stack Developer | 2 года | Astana |
| 4 | Мария Петрова | maria.mobile@example.com | Mobile Developer | 3 года | Almaty |
| 5 | Иван Кузнецов | ivan.datascience@example.com | Data Science / ML | 2 года | Astana |
| 6 | Ольга Морозова | olga.devops@example.com | DevOps / Cloud | 4 года | Almaty |
| 7 | Сергей Волков | sergey.qa@example.com | QA / Test Engineer | 2 года | Shymkent |
| 8 | Анна Лебедева | anna.designer@example.com | UI/UX + Frontend | 1 год | Almaty |
| 9 | Никита Сафин | nikita.database@example.com | Database / DBA | 5 лет | Almaty |
| 10 | Виктория Орлова | victoria.security@example.com | Security Engineer | 2 года | Almaty |

**Пароль для всех**: `password123`

## Использование

### Способ 1: Go скрипт (Предпочтительный)

```bash
cd marketplace-backend/cmd/seed
go run main.go
```

**Преимущества:**
- ✅ Автоматически шифрует личные данные
- ✅ Обрабатывает ошибки
- ✅ Показывает прогресс
- ✅ Работает с текущей конфигурацией БД

### Способ 2: SQL скрипт

```bash
# Подключиться к PostgreSQL
psql -h localhost -U postgres -d marketplace

# Выполнить скрипт
\i cmd/seed/seed_students.sql

# Или через psql напрямую
psql -h localhost -U postgres -d marketplace -f cmd/seed/seed_students.sql
```

## Проверка

После создания студентов, проверьте их:

```sql
SELECT u.email, sp.skills, sp.location, sp.experience_years 
FROM users u 
JOIN student_profiles sp ON u.id = sp.user_id 
WHERE u.role = 'student' 
ORDER BY u.created_at DESC 
LIMIT 10;
```

## Структура данных

Каждый студент содержит:
- **Email**: уникальный идентификатор
- **FullName**: зашифрованное полное имя
- **Phone**: зашифрованный номер телефона
- **Bio**: зашифрованная биография
- **Skills**: список навыков через запятую
- **Education**: информация об образовании
- **ExperienceYears**: количество лет опыта
- **Location**: город проживания
- **Availability**: тип доступности (remote, hybrid, onsite)

## Удаление созданных студентов

Если нужно удалить созданных студентов:

```sql
DELETE FROM student_profiles 
WHERE user_id IN (
  '550e8400-e29b-41d4-a716-446655440001',
  '550e8400-e29b-41d4-a716-446655440002',
  '550e8400-e29b-41d4-a716-446655440003',
  '550e8400-e29b-41d4-a716-446655440004',
  '550e8400-e29b-41d4-a716-446655440005',
  '550e8400-e29b-41d4-a716-446655440006',
  '550e8400-e29b-41d4-a716-446655440007',
  '550e8400-e29b-41d4-a716-446655440008',
  '550e8400-e29b-41d4-a716-446655440009',
  '550e8400-e29b-41d4-a716-446655440010'
);

DELETE FROM users 
WHERE email IN (
  'dmitry.frontend@example.com',
  'elena.backend@example.com',
  'alex.fullstack@example.com',
  'maria.mobile@example.com',
  'ivan.datascience@example.com',
  'olga.devops@example.com',
  'sergey.qa@example.com',
  'anna.designer@example.com',
  'nikita.database@example.com',
  'victoria.security@example.com'
);
```

## Специальности и навыки

### 1️⃣ Frontend Developer
```
React, Vue.js, TypeScript, CSS, HTML, Webpack, Redux
```

### 2️⃣ Backend Developer
```
Node.js, Python, Express, Django, PostgreSQL, MongoDB
```

### 3️⃣ Full Stack Developer
```
JavaScript, TypeScript, React, Node.js, PostgreSQL, Docker
```

### 4️⃣ Mobile Developer
```
Swift, Kotlin, React Native, Flutter, Firebase, iOS, Android
```

### 5️⃣ Data Science / ML Engineer
```
Python, TensorFlow, PyTorch, Pandas, NumPy, Scikit-learn, SQL
```

### 6️⃣ DevOps / Cloud Engineer
```
Docker, Kubernetes, AWS, CI/CD, Linux, Terraform, Git
```

### 7️⃣ QA / Test Engineer
```
Selenium, Manual Testing, Cucumber, JIRA, API Testing, Postman
```

### 8️⃣ UI/UX Designer + Frontend
```
Figma, JavaScript, React, CSS, UI Design, Prototyping, User Research
```

### 9️⃣ Database Engineer / DBA
```
PostgreSQL, MySQL, Oracle, SQL optimization, Replication, Backup, Performance Tuning
```

### 🔟 Security Engineer / Cybersecurity
```
Network Security, Penetration Testing, OWASP, Encryption, Linux, Docker
```
