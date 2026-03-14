package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
	"github.com/uni-intern-organization/marketplace-backend/config"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/db"
	"github.com/uni-intern-organization/marketplace-backend/internal/handler"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
	"github.com/uni-intern-organization/marketplace-backend/internal/storage"
)

var allRoles = []model.UserRole{model.RoleStudent, model.RoleRecruiter, model.RoleAdmin}

func main() {
	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	var pool *pgxpool.Pool
	for i := 0; i < 15; i++ {
		ctxConn, cancel := context.WithTimeout(ctx, 10*time.Second)
		pool, err = db.NewPool(ctxConn, &cfg.DB)
		cancel()
		if err == nil {
			break
		}
		log.Printf("db connect attempt %d: %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if pool == nil {
		log.Fatal("db: could not connect")
	}
	defer pool.Close()

	ctxMig, cancelMig := context.WithTimeout(ctx, 30*time.Second)
	defer cancelMig()
	if err := db.RunMigrations(ctxMig, pool); err != nil {
		log.Fatal("migrations:", err)
	}

	s3Storage, err := storage.NewS3Storage(&cfg.S3)
	if err != nil {
		log.Fatal("s3:", err)
	}
	for i := 0; i < 10; i++ {
		if err := s3Storage.EnsureBucket(ctx); err == nil {
			break
		}
		log.Printf("s3 bucket attempt %d: %v", i+1, err)
		time.Sleep(2 * time.Second)
	}

	aesKey := crypto.KeyFromString(cfg.AES.Key)

	userRepo := repository.NewUserRepository(pool)
	recruiterRepo := repository.NewRecruiterProfileRepository(pool)
	invRepo := repository.NewInvitationRepository(pool)
	appRepo := repository.NewApplicationRepository(pool)
	vacancyRepo := repository.NewVacancyRepository(pool)

	authHandler := handler.NewAuthHandler(userRepo, cfg.JWT.Secret, cfg.JWT.ExpireHours)
	profileHandler := handler.NewProfileHandler(userRepo, recruiterRepo, aesKey)
	fileHandler := handler.NewFileHandler(s3Storage, userRepo, recruiterRepo)
	invitationHandler := handler.NewInvitationHandler(invRepo, userRepo, aesKey)
	applicationHandler := handler.NewApplicationHandler(appRepo, invRepo, userRepo, vacancyRepo, aesKey)
	vacancyHandler := handler.NewVacancyHandler(vacancyRepo, userRepo, aesKey)
	matchHandler := handler.NewMatchHandler(vacancyRepo, userRepo, aesKey)
	searchHandler := handler.NewSearchHandler(pool, userRepo)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)

	authMiddleware := middleware.Auth(cfg.JWT.Secret)
	mux.Handle("GET /api/me", authMiddleware(middleware.RequireRole(allRoles...)(http.HandlerFunc(profileHandler.GetMyProfile))))
	mux.Handle("PUT /api/me/profile", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(profileHandler.UpdateStudentProfile))))
	mux.Handle("PATCH /api/me/profile", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(profileHandler.UpdateStudentProfile))))
	mux.Handle("PUT /api/me/recruiter", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(profileHandler.UpdateRecruiterProfile))))
	mux.Handle("PATCH /api/me/recruiter", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(profileHandler.UpdateRecruiterProfile))))
	mux.Handle("GET /api/users", authMiddleware(middleware.RequireRole(allRoles...)(http.HandlerFunc(profileHandler.GetUserByID))))
	mux.Handle("POST /api/files/resume", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(fileHandler.UploadResume))))
	mux.Handle("POST /api/files/logo", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(fileHandler.UploadCompanyLogo))))
	mux.Handle("GET /api/files/url", authMiddleware(middleware.RequireRole(allRoles...)(http.HandlerFunc(fileHandler.GetPresignedURL))))
	mux.Handle("POST /api/invitations", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(invitationHandler.Create))))
	mux.Handle("GET /api/invitations", authMiddleware(middleware.RequireRole(model.RoleStudent, model.RoleRecruiter)(http.HandlerFunc(invitationHandler.ListMine))))
	mux.Handle("PATCH /api/invitations", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(invitationHandler.UpdateStatus))))
	mux.Handle("POST /api/applications", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(applicationHandler.Create))))
	mux.Handle("GET /api/applications", authMiddleware(middleware.RequireRole(model.RoleStudent, model.RoleRecruiter)(http.HandlerFunc(applicationHandler.ListMine))))
	mux.Handle("PATCH /api/applications", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(applicationHandler.UpdateStatus))))
	mux.Handle("POST /api/vacancies", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(vacancyHandler.Create))))
	mux.Handle("GET /api/vacancies", authMiddleware(middleware.RequireRole(allRoles...)(http.HandlerFunc(vacancyHandler.GetOrList))))
	mux.Handle("GET /api/vacancies/mine", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(vacancyHandler.ListMine))))
	mux.Handle("PUT /api/vacancies", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(vacancyHandler.Update))))
	mux.Handle("PATCH /api/vacancies", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(vacancyHandler.Update))))
	mux.Handle("DELETE /api/vacancies", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(vacancyHandler.Delete))))
	mux.Handle("GET /api/match/vacancy", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(matchHandler.CandidatesForVacancy))))
	mux.Handle("GET /api/match/recommendations", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(matchHandler.RecommendationsForStudent))))
	mux.Handle("GET /api/search/users", authMiddleware(middleware.RequireRole(model.RoleAdmin, model.RoleRecruiter)(http.HandlerFunc(searchHandler.SearchUsers))))
	mux.Handle("GET /api/search/students", authMiddleware(middleware.RequireRole(model.RoleAdmin, model.RoleRecruiter)(http.HandlerFunc(searchHandler.SearchStudents))))
	mux.Handle("GET /api/students/{id}", authMiddleware(middleware.RequireRole(model.RoleAdmin, model.RoleRecruiter)(http.HandlerFunc(searchHandler.GetStudentByID))))

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}).Handler(mux)

	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: corsHandler,
	}
	go func() {
		log.Println("listening on :" + cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Println("shutdown:", err)
	}
}
