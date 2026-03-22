-- Seed script for 10 students with different specialties
-- Passwords are all "password123" hashed with bcrypt (pre-generated hashes)

INSERT INTO users (id, email, password_hash, role) VALUES
-- 1. Frontend Developer (React, Vue, Angular)
('550e8400-e29b-41d4-a716-446655440001', 'dmitry.frontend@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student'),

-- 2. Backend Developer (Node.js, Python, Go)
('550e8400-e29b-41d4-a716-446655440002', 'elena.backend@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student'),

-- 3. Full Stack Developer
('550e8400-e29b-41d4-a716-446655440003', 'alex.fullstack@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student'),

-- 4. Mobile Developer (iOS, Android)
('550e8400-e29b-41d4-a716-446655440004', 'maria.mobile@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student'),

-- 5. Data Science / ML Engineer
('550e8400-e29b-41d4-a716-446655440005', 'ivan.datascience@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student'),

-- 6. DevOps / Cloud Engineer
('550e8400-e29b-41d4-a716-446655440006', 'olga.devops@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student'),

-- 7. QA / Test Engineer
('550e8400-e29b-41d4-a716-446655440007', 'sergey.qa@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student'),

-- 8. UI/UX Designer + Frontend
('550e8400-e29b-41d4-a716-446655440008', 'anna.designer@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student'),

-- 9. Database Engineer / DBA
('550e8400-e29b-41d4-a716-446655440009', 'nikita.database@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student'),

-- 10. Security Engineer / Cybersecurity
('550e8400-e29b-41d4-a716-446655440010', 'victoria.security@example.com', '$2a$10$N9qo8ucoINH87is200oP7OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW', 'student')
ON CONFLICT (email) DO NOTHING;

-- Add student profiles with different specialties
INSERT INTO student_profiles (user_id, skills, education, experience_years, location, availability) VALUES

-- 1. Frontend Developer
('550e8400-e29b-41d4-a716-446655440001', 'React, Vue.js, TypeScript, CSS, HTML, Webpack, Redux', 'Bachelor of Computer Science - Al-Farabi KNU', 2, 'Almaty', 'remote'),

-- 2. Backend Developer
('550e8400-e29b-41d4-a716-446655440002', 'Node.js, Python, Express, Django, PostgreSQL, MongoDB', 'Bachelor of Software Engineering - KazNU', 3, 'Almaty', 'hybrid'),

-- 3. Full Stack Developer
('550e8400-e29b-41d4-a716-446655440003', 'JavaScript, TypeScript, React, Node.js, PostgreSQL, Docker', 'Bachelor of IT - Astra University', 2, 'Astana', 'remote'),

-- 4. Mobile Developer
('550e8400-e29b-41d4-a716-446655440004', 'Swift, Kotlin, React Native, Flutter, Firebase, iOS, Android', 'Bachelor of Computer Engineering - KIMEP', 3, 'Almaty', 'onsite'),

-- 5. Data Science / ML Engineer
('550e8400-e29b-41d4-a716-446655440005', 'Python, TensorFlow, PyTorch, Pandas, NumPy, Scikit-learn, SQL', 'Master of Data Science - Nazarbayev University', 2, 'Astana', 'hybrid'),

-- 6. DevOps / Cloud Engineer
('550e8400-e29b-41d4-a716-446655440006', 'Docker, Kubernetes, AWS, CI/CD, Linux, Terraform, Git', 'Bachelor of Information Technology - TSU', 4, 'Almaty', 'remote'),

-- 7. QA / Test Engineer
('550e8400-e29b-41d4-a716-446655440007', 'Selenium, Manual Testing, Cucumber, JIRA, API Testing, Postman', 'Bachelor of Software Testing - KazIT University', 2, 'Shymkent', 'hybrid'),

-- 8. UI/UX Designer + Frontend
('550e8400-e29b-41d4-a716-446655440008', 'Figma, JavaScript, React, CSS, UI Design, Prototyping, User Research', 'Bachelor of Design Engineering - KAFU', 1, 'Almaty', 'remote'),

-- 9. Database Engineer / DBA
('550e8400-e29b-41d4-a716-446655440009', 'PostgreSQL, MySQL, Oracle, SQL optimization, Replication, Backup, Performance Tuning', 'Bachelor of Database Administration - AITU', 5, 'Almaty', 'onsite'),

-- 10. Security Engineer / Cybersecurity
('550e8400-e29b-41d4-a716-446655440010', 'Network Security, Penetration Testing, OWASP, Encryption, Linux, Docker', 'Bachelor of Cybersecurity - Suleyman Demirel University', 2, 'Almaty', 'hybrid')
ON CONFLICT (user_id) DO NOTHING;

-- Return created user count
SELECT COUNT(*) as created_students FROM student_profiles WHERE user_id IN (
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
