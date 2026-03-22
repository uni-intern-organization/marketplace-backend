-- ============================================
-- Скрипт создания 10 студентов с разными специальностями
-- Пароль для всех: password123
-- Требует extensions: pgcrypto для gen_random_uuid()
-- ============================================

-- Убедитесь, что расширение pgcrypto установлено
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ============================================
-- 1. ДМИТРИЙ ИВАНОВ - Frontend Developer
-- ============================================
INSERT INTO users (email, password_hash, role) 
VALUES ('dmitry.frontend@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student')
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (user_id, skills, education, experience_years, location, availability)
SELECT id, 
  'React, Vue.js, TypeScript, CSS, HTML, Webpack, Redux',
  'Bachelor of Computer Science - Al-Farabi KNU',
  2,
  'Almaty',
  'remote'
FROM users WHERE email = 'dmitry.frontend@example.com'
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 2. ЕЛЕНА КАРТУЗОВА - Backend Developer
-- ============================================
INSERT INTO users (email, password_hash, role)
VALUES ('elena.backend@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student')
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (user_id, skills, education, experience_years, location, availability)
SELECT id,
  'Node.js, Python, Express, Django, PostgreSQL, MongoDB',
  'Bachelor of Software Engineering - KazNU',
  3,
  'Almaty',
  'hybrid'
FROM users WHERE email = 'elena.backend@example.com'
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 3. АЛЕКСАНДР СМИРНОВ - Full Stack Developer
-- ============================================
INSERT INTO users (email, password_hash, role)
VALUES ('alex.fullstack@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student')
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (user_id, skills, education, experience_years, location, availability)
SELECT id,
  'JavaScript, TypeScript, React, Node.js, PostgreSQL, Docker',
  'Bachelor of IT - Astra University',
  2,
  'Astana',
  'remote'
FROM users WHERE email = 'alex.fullstack@example.com'
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 4. МАРИЯ ПЕТРОВА - Mobile Developer
-- ============================================
INSERT INTO users (email, password_hash, role)
VALUES ('maria.mobile@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student')
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (user_id, skills, education, experience_years, location, availability)
SELECT id,
  'Swift, Kotlin, React Native, Flutter, Firebase, iOS, Android',
  'Bachelor of Computer Engineering - KIMEP',
  3,
  'Almaty',
  'onsite'
FROM users WHERE email = 'maria.mobile@example.com'
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 5. ИВАН КУЗНЕЦОВ - Data Science / ML Engineer
-- ============================================
INSERT INTO users (email, password_hash, role)
VALUES ('ivan.datascience@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student')
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (user_id, skills, education, experience_years, location, availability)
SELECT id,
  'Python, TensorFlow, PyTorch, Pandas, NumPy, Scikit-learn, SQL',
  'Master of Data Science - Nazarbayev University',
  2,
  'Astana',
  'hybrid'
FROM users WHERE email = 'ivan.datascience@example.com'
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 6. ОЛЬГА МОРОЗОВА - DevOps / Cloud Engineer
-- ============================================
INSERT INTO users (email, password_hash, role)
VALUES ('olga.devops@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student')
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (user_id, skills, education, experience_years, location, availability)
SELECT id,
  'Docker, Kubernetes, AWS, CI/CD, Linux, Terraform, Git',
  'Bachelor of Information Technology - TSU',
  4,
  'Almaty',
  'remote'
FROM users WHERE email = 'olga.devops@example.com'
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 7. СЕРГЕЙ ВОЛКОВ - QA / Test Engineer
-- ============================================
INSERT INTO users (email, password_hash, role)
VALUES ('sergey.qa@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student')
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (user_id, skills, education, experience_years, location, availability)
SELECT id,
  'Selenium, Manual Testing, Cucumber, JIRA, API Testing, Postman',
  'Bachelor of Software Testing - KazIT University',
  2,
  'Shymkent',
  'hybrid'
FROM users WHERE email = 'sergey.qa@example.com'
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 8. АННА ЛЕБЕДЕВА - UI/UX Designer + Frontend
-- ============================================
INSERT INTO users (email, password_hash, role)
VALUES ('anna.designer@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student')
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (user_id, skills, education, experience_years, location, availability)
SELECT id,
  'Figma, JavaScript, React, CSS, UI Design, Prototyping, User Research',
  'Bachelor of Design Engineering - KAFU',
  1,
  'Almaty',
  'remote'
FROM users WHERE email = 'anna.designer@example.com'
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 9. НИКИТА САФИН - Database Engineer / DBA
-- ============================================
INSERT INTO users (email, password_hash, role)
VALUES ('nikita.database@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student')
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (user_id, skills, education, experience_years, location, availability)
SELECT id,
  'PostgreSQL, MySQL, Oracle, SQL optimization, Replication, Backup, Performance Tuning',
  'Bachelor of Database Administration - AITU',
  5,
  'Almaty',
  'onsite'
FROM users WHERE email = 'nikita.database@example.com'
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 10. ВИКТОРИЯ ОРЛОВА - Security Engineer
-- ============================================
INSERT INTO users (email, password_hash, role)
VALUES ('victoria.security@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student')
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (user_id, skills, education, experience_years, location, availability)
SELECT id,
  'Network Security, Penetration Testing, OWASP, Encryption, Linux, Docker',
  'Bachelor of Cybersecurity - Suleyman Demirel University',
  2,
  'Almaty',
  'hybrid'
FROM users WHERE email = 'victoria.security@example.com'
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- ПРОВЕРКА: Просмотр созданных студентов
-- ============================================
SELECT 
  u.email,
  sp.skills,
  sp.education,
  sp.experience_years,
  sp.location,
  sp.availability
FROM users u
LEFT JOIN student_profiles sp ON u.id = sp.user_id
WHERE u.role = 'student'
  AND u.email IN (
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
  )
ORDER BY u.email;
