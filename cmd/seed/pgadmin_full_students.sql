-- ============================================
-- ПОЛНЫЙ СКРИПТ: 10 студентов с полными данными
-- Для использования в pgAdmin
-- Пароль для всех: password123
-- ============================================

-- Убедитесь, что расширение pgcrypto установлено
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ============================================
-- 1. ДМИТРИЙ ИВАНОВ - Frontend Developer
-- Email: dmitry.frontend@example.com
-- ============================================
INSERT INTO users (email, password_hash, role, created_at, updated_at)
VALUES (
  'dmitry.frontend@example.com',
  '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW',
  'student',
  NOW(),
  NOW()
)
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (
  user_id, full_name_enc, phone_enc, bio_enc, 
  skills, education, experience_years, location, availability,
  created_at, updated_at
)
SELECT
  id,
  pgp_sym_encrypt('Дмитрий Иванов', 'secret_key'),
  pgp_sym_encrypt('+7 702 123 4501', 'secret_key'),
  pgp_sym_encrypt('Молодой Frontend разработчик с 2 годами опыта в разработке веб-приложений с использованием React и Vue.js', 'secret_key'),
  'React, Vue.js, TypeScript, CSS, HTML, Webpack, Redux',
  'Bachelor of Computer Science - Al-Farabi KNU (2022-2024)',
  2,
  'Almaty',
  'remote'
FROM users
WHERE email = 'dmitry.frontend@example.com'
  AND NOT EXISTS (SELECT 1 FROM student_profiles WHERE user_id = users.id)
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 2. ЕЛЕНА КАРТУЗОВА - Backend Developer
-- Email: elena.backend@example.com
-- ============================================
INSERT INTO users (email, password_hash, role, created_at, updated_at)
VALUES (
  'elena.backend@example.com',
  '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW',
  'student',
  NOW(),
  NOW()
)
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (
  user_id, full_name_enc, phone_enc, bio_enc,
  skills, education, experience_years, location, availability,
  created_at, updated_at
)
SELECT
  id,
  pgp_sym_encrypt('Елена Картузова', 'secret_key'),
  pgp_sym_encrypt('+7 702 123 4502', 'secret_key'),
  pgp_sym_encrypt('Опытный backend разработчик с 3 годами разработки серверной части приложений. Специализируюсь на Node.js и Python', 'secret_key'),
  'Node.js, Python, Express, Django, PostgreSQL, MongoDB',
  'Bachelor of Software Engineering - KazNU (2021-2023)',
  3,
  'Almaty',
  'hybrid'
FROM users
WHERE email = 'elena.backend@example.com'
  AND NOT EXISTS (SELECT 1 FROM student_profiles WHERE user_id = users.id)
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 3. АЛЕКСАНДР СМИРНОВ - Full Stack Developer
-- Email: alex.fullstack@example.com
-- ============================================
INSERT INTO users (email, password_hash, role, created_at, updated_at)
VALUES (
  'alex.fullstack@example.com',
  '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW',
  'student',
  NOW(),
  NOW()
)
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (
  user_id, full_name_enc, phone_enc, bio_enc,
  skills, education, experience_years, location, availability,
  created_at, updated_at
)
SELECT
  id,
  pgp_sym_encrypt('Александр Смирнов', 'secret_key'),
  pgp_sym_encrypt('+7 702 123 4503', 'secret_key'),
  pgp_sym_encrypt('Full stack разработчик, разбираюсь как во фронтенде так и в бэкенде. Опыт работы с современными фреймворками', 'secret_key'),
  'JavaScript, TypeScript, React, Node.js, PostgreSQL, Docker',
  'Bachelor of IT - Astra University (2022-2024)',
  2,
  'Astana',
  'remote'
FROM users
WHERE email = 'alex.fullstack@example.com'
  AND NOT EXISTS (SELECT 1 FROM student_profiles WHERE user_id = users.id)
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 4. МАРИЯ ПЕТРОВА - Mobile Developer
-- Email: maria.mobile@example.com
-- ============================================
INSERT INTO users (email, password_hash, role, created_at, updated_at)
VALUES (
  'maria.mobile@example.com',
  '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW',
  'student',
  NOW(),
  NOW()
)
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (
  user_id, full_name_enc, phone_enc, bio_enc,
  skills, education, experience_years, location, availability,
  created_at, updated_at
)
SELECT
  id,
  pgp_sym_encrypt('Мария Петрова', 'secret_key'),
  pgp_sym_encrypt('+7 702 123 4504', 'secret_key'),
  pgp_sym_encrypt('Мобильный разработчик с опытом iOS и Android разработки. Создаю кроссплатформенные приложения', 'secret_key'),
  'Swift, Kotlin, React Native, Flutter, Firebase, iOS, Android',
  'Bachelor of Computer Engineering - KIMEP (2021-2023)',
  3,
  'Almaty',
  'onsite'
FROM users
WHERE email = 'maria.mobile@example.com'
  AND NOT EXISTS (SELECT 1 FROM student_profiles WHERE user_id = users.id)
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 5. ИВАН КУЗНЕЦОВ - Data Science / ML Engineer
-- Email: ivan.datascience@example.com
-- ============================================
INSERT INTO users (email, password_hash, role, created_at, updated_at)
VALUES (
  'ivan.datascience@example.com',
  '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW',
  'student',
  NOW(),
  NOW()
)
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (
  user_id, full_name_enc, phone_enc, bio_enc,
  skills, education, experience_years, location, availability,
  created_at, updated_at
)
SELECT
  id,
  pgp_sym_encrypt('Иван Кузнецов', 'secret_key'),
  pgp_sym_encrypt('+7 702 123 4505', 'secret_key'),
  pgp_sym_encrypt('Специалист в области Data Science и машинного обучения. Работаю с большими данными и нейросетями', 'secret_key'),
  'Python, TensorFlow, PyTorch, Pandas, NumPy, Scikit-learn, SQL',
  'Master of Data Science - Nazarbayev University (2023-2024)',
  2,
  'Astana',
  'hybrid'
FROM users
WHERE email = 'ivan.datascience@example.com'
  AND NOT EXISTS (SELECT 1 FROM student_profiles WHERE user_id = users.id)
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 6. ОЛЬГА МОРОЗОВА - DevOps / Cloud Engineer
-- Email: olga.devops@example.com
-- ============================================
INSERT INTO users (email, password_hash, role, created_at, updated_at)
VALUES (
  'olga.devops@example.com',
  '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW',
  'student',
  NOW(),
  NOW()
)
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (
  user_id, full_name_enc, phone_enc, bio_enc,
  skills, education, experience_years, location, availability,
  created_at, updated_at
)
SELECT
  id,
  pgp_sym_encrypt('Ольга Морозова', 'secret_key'),
  pgp_sym_encrypt('+7 702 123 4506', 'secret_key'),
  pgp_sym_encrypt('DevOps инженер с опытом работы с облачными платформами AWS и управлением инфраструктурой', 'secret_key'),
  'Docker, Kubernetes, AWS, CI/CD, Linux, Terraform, Git',
  'Bachelor of Information Technology - TSU (2020-2022)',
  4,
  'Almaty',
  'remote'
FROM users
WHERE email = 'olga.devops@example.com'
  AND NOT EXISTS (SELECT 1 FROM student_profiles WHERE user_id = users.id)
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 7. СЕРГЕЙ ВОЛКОВ - QA / Test Engineer
-- Email: sergey.qa@example.com
-- ============================================
INSERT INTO users (email, password_hash, role, created_at, updated_at)
VALUES (
  'sergey.qa@example.com',
  '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW',
  'student',
  NOW(),
  NOW()
)
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (
  user_id, full_name_enc, phone_enc, bio_enc,
  skills, education, experience_years, location, availability,
  created_at, updated_at
)
SELECT
  id,
  pgp_sym_encrypt('Сергей Волков', 'secret_key'),
  pgp_sym_encrypt('+7 702 123 4507', 'secret_key'),
  pgp_sym_encrypt('QA инженер с опытом автоматизированного и ручного тестирования веб и мобильных приложений', 'secret_key'),
  'Selenium, Manual Testing, Cucumber, JIRA, API Testing, Postman',
  'Bachelor of Software Testing - KazIT University (2022-2024)',
  2,
  'Shymkent',
  'hybrid'
FROM users
WHERE email = 'sergey.qa@example.com'
  AND NOT EXISTS (SELECT 1 FROM student_profiles WHERE user_id = users.id)
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 8. АННА ЛЕБЕДЕВА - UI/UX Designer + Frontend
-- Email: anna.designer@example.com
-- ============================================
INSERT INTO users (email, password_hash, role, created_at, updated_at)
VALUES (
  'anna.designer@example.com',
  '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW',
  'student',
  NOW(),
  NOW()
)
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (
  user_id, full_name_enc, phone_enc, bio_enc,
  skills, education, experience_years, location, availability,
  created_at, updated_at
)
SELECT
  id,
  pgp_sym_encrypt('Анна Лебедева', 'secret_key'),
  pgp_sym_encrypt('+7 702 123 4508', 'secret_key'),
  pgp_sym_encrypt('UI/UX дизайнер и фронтенд разработчик. Создаю красивые и удобные пользовательские интерфейсы', 'secret_key'),
  'Figma, JavaScript, React, CSS, UI Design, Prototyping, User Research',
  'Bachelor of Design Engineering - KAFU (2023-2024)',
  1,
  'Almaty',
  'remote'
FROM users
WHERE email = 'anna.designer@example.com'
  AND NOT EXISTS (SELECT 1 FROM student_profiles WHERE user_id = users.id)
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 9. НИКИТА САФИН - Database Engineer / DBA
-- Email: nikita.database@example.com
-- ============================================
INSERT INTO users (email, password_hash, role, created_at, updated_at)
VALUES (
  'nikita.database@example.com',
  '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW',
  'student',
  NOW(),
  NOW()
)
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (
  user_id, full_name_enc, phone_enc, bio_enc,
  skills, education, experience_years, location, availability,
  created_at, updated_at
)
SELECT
  id,
  pgp_sym_encrypt('Никита Сафин', 'secret_key'),
  pgp_sym_encrypt('+7 702 123 4509', 'secret_key'),
  pgp_sym_encrypt('Database Engineer с опытом администрирования и оптимизации баз данных. Специализируюсь на PostgreSQL', 'secret_key'),
  'PostgreSQL, MySQL, Oracle, SQL optimization, Replication, Backup, Performance Tuning',
  'Bachelor of Database Administration - AITU (2019-2021)',
  5,
  'Almaty',
  'onsite'
FROM users
WHERE email = 'nikita.database@example.com'
  AND NOT EXISTS (SELECT 1 FROM student_profiles WHERE user_id = users.id)
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- 10. ВИКТОРИЯ ОРЛОВА - Security Engineer
-- Email: victoria.security@example.com
-- ============================================
INSERT INTO users (email, password_hash, role, created_at, updated_at)
VALUES (
  'victoria.security@example.com',
  '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW',
  'student',
  NOW(),
  NOW()
)
ON CONFLICT (email) DO NOTHING;

INSERT INTO student_profiles (
  user_id, full_name_enc, phone_enc, bio_enc,
  skills, education, experience_years, location, availability,
  created_at, updated_at
)
SELECT
  id,
  pgp_sym_encrypt('Виктория Орлова', 'secret_key'),
  pgp_sym_encrypt('+7 702 123 4510', 'secret_key'),
  pgp_sym_encrypt('Security Engineer и специалист по кибербезопасности. Провожу тестирование на уязвимости', 'secret_key'),
  'Network Security, Penetration Testing, OWASP, Encryption, Linux, Docker',
  'Bachelor of Cybersecurity - Suleyman Demirel University (2022-2024)',
  2,
  'Almaty',
  'hybrid'
FROM users
WHERE email = 'victoria.security@example.com'
  AND NOT EXISTS (SELECT 1 FROM student_profiles WHERE user_id = users.id)
ON CONFLICT (user_id) DO NOTHING;

-- ============================================
-- ПРОВЕРКА: Просмотр всех созданных студентов
-- ============================================
SELECT 
  u.id,
  u.email,
  u.role,
  sp.skills,
  sp.education,
  sp.experience_years,
  sp.location,
  sp.availability,
  u.created_at
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
ORDER BY u.created_at DESC;
