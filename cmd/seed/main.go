package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uni-intern-organization/marketplace-backend/config"
	"github.com/uni-intern-organization/marketplace-backend/internal/auth"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/db"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

type StudentData struct {
	Email          string
	FullName       string
	Phone          string
	Bio            string
	Skills         string
	Education      string
	ExperienceYears int
	Location       string
	Availability   string
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Ошибка при загрузке конфига:", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, &cfg.DB)
	if err != nil {
		log.Fatal("Ошибка подключения к БД:", err)
	}
	defer pool.Close()

	aesKey := crypto.KeyFromString(cfg.AES.Key)

	students := []StudentData{
		{
			Email:           "dmitry.frontend@example.com",
			FullName:        "Дмитрий Иванов",
			Phone:           "+7 702 123 4501",
			Bio:             "Frontend разработчик с 2 годами опыта",
			Skills:          "React, Vue.js, TypeScript, CSS, HTML, Webpack, Redux",
			Education:       "Bachelor of Computer Science - Al-Farabi KNU",
			ExperienceYears: 2,
			Location:        "Almaty",
			Availability:    "remote",
		},
		{
			Email:           "elena.backend@example.com",
			FullName:        "Елена Картузова",
			Phone:           "+7 702 123 4502",
			Bio:             "Опытный backend разработчик",
			Skills:          "Node.js, Python, Express, Django, PostgreSQL, MongoDB",
			Education:       "Bachelor of Software Engineering - KazNU",
			ExperienceYears: 3,
			Location:        "Almaty",
			Availability:    "hybrid",
		},
		{
			Email:           "alex.fullstack@example.com",
			FullName:        "Александр Смирнов",
			Phone:           "+7 702 123 4503",
			Bio:             "Full stack разработчик",
			Skills:          "JavaScript, TypeScript, React, Node.js, PostgreSQL, Docker",
			Education:       "Bachelor of IT - Astra University",
			ExperienceYears: 2,
			Location:        "Astana",
			Availability:    "remote",
		},
		{
			Email:           "maria.mobile@example.com",
			FullName:        "Мария Петрова",
			Phone:           "+7 702 123 4504",
			Bio:             "Mobile разработчик iOS и Android",
			Skills:          "Swift, Kotlin, React Native, Flutter, Firebase, iOS, Android",
			Education:       "Bachelor of Computer Engineering - KIMEP",
			ExperienceYears: 3,
			Location:        "Almaty",
			Availability:    "onsite",
		},
		{
			Email:           "ivan.datascience@example.com",
			FullName:        "Иван Кузнецов",
			Phone:           "+7 702 123 4505",
			Bio:             "Data Scientist and ML Engineer",
			Skills:          "Python, TensorFlow, PyTorch, Pandas, NumPy, Scikit-learn, SQL",
			Education:       "Master of Data Science - Nazarbayev University",
			ExperienceYears: 2,
			Location:        "Astana",
			Availability:    "hybrid",
		},
		{
			Email:           "olga.devops@example.com",
			FullName:        "Ольга Морозова",
			Phone:           "+7 702 123 4506",
			Bio:             "DevOps и Cloud инженер",
			Skills:          "Docker, Kubernetes, AWS, CI/CD, Linux, Terraform, Git",
			Education:       "Bachelor of Information Technology - TSU",
			ExperienceYears: 4,
			Location:        "Almaty",
			Availability:    "remote",
		},
		{
			Email:           "sergey.qa@example.com",
			FullName:        "Сергей Волков",
			Phone:           "+7 702 123 4507",
			Bio:             "QA инженер с опытом тестирования",
			Skills:          "Selenium, Manual Testing, Cucumber, JIRA, API Testing, Postman",
			Education:       "Bachelor of Software Testing - KazIT University",
			ExperienceYears: 2,
			Location:        "Shymkent",
			Availability:    "hybrid",
		},
		{
			Email:           "anna.designer@example.com",
			FullName:        "Анна Лебедева",
			Phone:           "+7 702 123 4508",
			Bio:             "UI/UX дизайнер и фронтенд разработчик",
			Skills:          "Figma, JavaScript, React, CSS, UI Design, Prototyping, User Research",
			Education:       "Bachelor of Design Engineering - KAFU",
			ExperienceYears: 1,
			Location:        "Almaty",
			Availability:    "remote",
		},
		{
			Email:           "nikita.database@example.com",
			FullName:        "Никита Сафин",
			Phone:           "+7 702 123 4509",
			Bio:             "Database Engineer и DBA",
			Skills:          "PostgreSQL, MySQL, Oracle, SQL optimization, Replication, Backup, Performance Tuning",
			Education:       "Bachelor of Database Administration - AITU",
			ExperienceYears: 5,
			Location:        "Almaty",
			Availability:    "onsite",
		},
		{
			Email:           "victoria.security@example.com",
			FullName:        "Виктория Орлова",
			Phone:           "+7 702 123 4510",
			Bio:             "Security Engineer и специалист по кибербезопасности",
			Skills:          "Network Security, Penetration Testing, OWASP, Encryption, Linux, Docker",
			Education:       "Bachelor of Cybersecurity - Suleyman Demirel University",
			ExperienceYears: 2,
			Location:        "Almaty",
			Availability:    "hybrid",
		},
	}

	createdCount := 0
	for i, student := range students {
		userID := uuid.New()

		// Hash password
		passwordHash, err := auth.HashPassword("password123")
		if err != nil {
			log.Printf("[%d] Ошибка хеширования пароля: %v\n", i+1, err)
			continue
		}

		// Insert user
		_, err = pool.Exec(ctx, `
			INSERT INTO users (id, email, password_hash, role, created_at, updated_at)
			VALUES ($1, $2, $3, $4, NOW(), NOW())
			ON CONFLICT (email) DO NOTHING
		`, userID, student.Email, passwordHash, model.RoleStudent)
		if err != nil {
			log.Printf("[%d] Ошибка создания пользователя %s: %v\n", i+1, student.Email, err)
			continue
		}

		// Encrypt sensitive data
		fullNameEnc := crypto.Encrypt(student.FullName, aesKey)
		phoneEnc := crypto.Encrypt(student.Phone, aesKey)
		bioEnc := crypto.Encrypt(student.Bio, aesKey)

		// Insert student profile
		_, err = pool.Exec(ctx, `
			INSERT INTO student_profiles (
				user_id, full_name_enc, phone_enc, bio_enc, skills, education,
				experience_years, location, availability, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
			ON CONFLICT (user_id) DO NOTHING
		`, userID, fullNameEnc, phoneEnc, bioEnc, student.Skills, student.Education,
			student.ExperienceYears, student.Location, student.Availability)
		if err != nil {
			log.Printf("[%d] Ошибка создания профиля студента: %v\n", i+1, err)
			continue
		}

		createdCount++
		fmt.Printf("✓ [%d] Создан студент: %s (%s)\n", i+1, student.FullName, student.Email)
	}

	fmt.Printf("\n✅ Успешно создано студентов: %d / %d\n", createdCount, len(students))
}
