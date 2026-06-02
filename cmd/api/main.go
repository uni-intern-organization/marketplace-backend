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
	"github.com/rs/cors"
	"github.com/uni-intern-organization/marketplace-backend/config"
	"github.com/uni-intern-organization/marketplace-backend/internal/billing"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/db"
	"github.com/uni-intern-organization/marketplace-backend/internal/email"
	"github.com/uni-intern-organization/marketplace-backend/internal/handler"
	"github.com/uni-intern-organization/marketplace-backend/internal/jobs"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/notifier"
	"github.com/uni-intern-organization/marketplace-backend/internal/payment"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
	"github.com/uni-intern-organization/marketplace-backend/internal/storage"
)

var allRoles = []model.UserRole{model.RoleStudent, model.RoleRecruiter, model.RoleModerator, model.RoleAdmin}
var modRoles = []model.UserRole{model.RoleModerator, model.RoleAdmin}

func main() {
	log.SetFlags(log.LstdFlags)
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	cfg.LogSummary()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var pool *pgxpool.Pool
	for i := 0; i < 15; i++ {
		ctxConn, cancelConn := context.WithTimeout(ctx, 10*time.Second)
		pool, err = db.NewPool(ctxConn, &cfg.DB)
		cancelConn()
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
	if err := db.RunMigrations(ctxMig, pool); err != nil {
		cancelMig()
		log.Fatal("migrations:", err)
	}
	cancelMig()
	log.Println("db: migrations applied")

	jobs.StartVacancyArchiver(ctx, pool, time.Hour)

	s3Storage, err := storage.NewS3Storage(&cfg.S3)
	if err != nil {
		log.Fatal("s3:", err)
	}
	s3Ready := false
	for i := 0; i < 10; i++ {
		if err := s3Storage.EnsureBucket(ctx); err == nil {
			s3Ready = true
			log.Println("s3: bucket ready")
			break
		}
		log.Printf("s3 bucket attempt %d: %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if !s3Ready {
		log.Println("s3: WARNING — bucket not ready")
	}

	aesKey := crypto.KeyFromString(cfg.AES.Key)
	emailSvc := email.NewService(&cfg.SMTP)

	userRepo := repository.NewUserRepository(pool)
	recruiterRepo := repository.NewRecruiterProfileRepository(pool)
	invRepo := repository.NewInvitationRepository(pool)
	appRepo := repository.NewApplicationRepository(pool)
	vacancyRepo := repository.NewVacancyRepository(pool)
	billingRepo := repository.NewBillingRepository(pool)
	paymentRepo := repository.NewPaymentRepository(pool)
	freelanceRepo := repository.NewFreelanceRepository(pool)
	hackathonRepo := repository.NewHackathonRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	notifRepo := repository.NewNotificationRepository(pool)
	msgRepo := repository.NewMessagingRepository(pool)
	walletRepo := repository.NewWalletRepository(pool)
	modRepo := repository.NewModerationRepository(pool)
	authSecRepo := repository.NewAuthSecurityRepository(pool)
	verificationRepo := repository.NewVerificationRepository(pool)

	billingSvc := billing.NewService(recruiterRepo, vacancyRepo).WithModeration(modRepo)
	notifierSvc := notifier.NewService(notifRepo, billingRepo, emailSvc, userRepo)
	jobs.StartScheduler(ctx, pool, notifierSvc)

	paymentProv := payment.NewDemoProvider(paymentRepo)
	paymentWebhook := payment.NewWebhookHandler(paymentProv, paymentRepo)

	authHandler := handler.NewAuthHandler(userRepo, authSecRepo, auditRepo, emailSvc, aesKey, cfg)
	profileHandler := handler.NewProfileHandler(userRepo, recruiterRepo, billingSvc, notifRepo, aesKey)
	fileHandler := handler.NewFileHandler(s3Storage, userRepo, recruiterRepo)
	invitationHandler := handler.NewInvitationHandler(invRepo, userRepo, billingSvc, aesKey)
	applicationHandler := handler.NewApplicationHandler(appRepo, invRepo, userRepo, vacancyRepo, aesKey)
	vacancyHandler := handler.NewVacancyHandler(vacancyRepo, userRepo, billingSvc, aesKey)
	vacancyExtHandler := handler.NewVacancyExtHandler(vacancyRepo, aesKey)
	matchHandler := handler.NewMatchHandler(vacancyRepo, freelanceRepo, hackathonRepo, userRepo, billingSvc, aesKey)
	searchHandler := handler.NewSearchHandler(pool, userRepo, billingSvc, vacancyRepo, freelanceRepo, hackathonRepo, aesKey)
	billingHandler := handler.NewBillingHandler(billingRepo, recruiterRepo, billingSvc, vacancyRepo, paymentRepo, paymentProv, &cfg.Billing)
	freelanceHandler := handler.NewFreelanceHandler(freelanceRepo, billingRepo, &cfg.Billing, aesKey)
	hackathonHandler := handler.NewHackathonHandler(hackathonRepo, billingRepo, &cfg.Billing, aesKey)
	portfolioHandler := handler.NewPortfolioHandler(appRepo, freelanceRepo, hackathonRepo, billingSvc)
	modHandler := handler.NewModerationHandler(modRepo, vacancyRepo, hackathonRepo, auditRepo, notifierSvc, aesKey)
	msgHandler := handler.NewMessagingHandler(msgRepo, notifierSvc, s3Storage, aesKey)
	notifHandler := handler.NewNotificationHandler(notifRepo, paymentRepo)
	walletHandler := handler.NewWalletHandler(walletRepo, notifierSvc)
	adminHandler := handler.NewAdminHandler(userRepo, walletRepo, auditRepo, billingRepo, paymentRepo)
	aiHandler := handler.NewAIHandler(billingSvc, vacancyRepo, userRepo)
	verificationHandler := handler.NewVerificationHandler(verificationRepo, auditRepo)
	publicHandler := handler.NewPublicHandler(paymentRepo)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/auth/refresh", authHandler.Refresh)
	mux.HandleFunc("POST /api/auth/forgot-password", authHandler.ForgotPassword)
	mux.HandleFunc("POST /api/auth/reset-password", authHandler.ResetPassword)

	mux.HandleFunc("GET /api/public/stats", publicHandler.Stats)
	mux.HandleFunc("POST /api/payments/webhook", paymentWebhook.HandleCheckoutConfirm)

	authMiddleware := middleware.Auth(cfg.JWT.Secret)
	mux.Handle("POST /api/auth/logout", authMiddleware(http.HandlerFunc(authHandler.Logout)))
	mux.Handle("POST /api/auth/2fa/setup", authMiddleware(http.HandlerFunc(authHandler.Setup2FA)))
	mux.Handle("POST /api/auth/2fa/verify", authMiddleware(http.HandlerFunc(authHandler.Verify2FA)))

	mux.Handle("GET /api/me", authMiddleware(middleware.RequireRole(allRoles...)(http.HandlerFunc(profileHandler.GetMyProfile))))
	mux.Handle("PUT /api/me/profile", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(profileHandler.UpdateStudentProfile))))
	mux.Handle("PATCH /api/me/profile", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(profileHandler.UpdateStudentProfile))))
	mux.Handle("PUT /api/me/recruiter", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(profileHandler.UpdateRecruiterProfile))))
	mux.Handle("PATCH /api/me/recruiter", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(profileHandler.UpdateRecruiterProfile))))
	mux.Handle("GET /api/me/profile/completion", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(profileHandler.GetProfileCompletion))))
	mux.Handle("GET /api/me/activity", authMiddleware(middleware.RequireRole(allRoles...)(http.HandlerFunc(profileHandler.GetActivity))))
	mux.Handle("GET /api/me/profile/visibility", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(profileHandler.GetProfileVisibility))))
	mux.Handle("PATCH /api/me/profile/visibility", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(profileHandler.UpdateProfileVisibility))))
	mux.Handle("POST /api/me/recruiter/verification", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(verificationHandler.Submit))))
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
	mux.Handle("POST /api/vacancies/draft", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(vacancyExtHandler.SaveDraft))))
	mux.Handle("GET /api/vacancies", http.HandlerFunc(vacancyHandler.GetOrList))
	mux.Handle("GET /api/vacancies/mine", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(vacancyHandler.ListMine))))
	mux.Handle("GET /api/vacancies/favorites", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(vacancyExtHandler.ListFavorites))))
	mux.Handle("POST /api/vacancies/favorites", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(vacancyExtHandler.AddFavorite))))
	mux.Handle("DELETE /api/vacancies/favorites", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(vacancyExtHandler.RemoveFavorite))))
	mux.Handle("POST /api/vacancies/renew", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(vacancyHandler.Renew))))
	mux.Handle("PUT /api/vacancies", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(vacancyHandler.Update))))
	mux.Handle("PATCH /api/vacancies", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(vacancyHandler.Update))))
	mux.Handle("DELETE /api/vacancies", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(vacancyHandler.Delete))))

	mux.Handle("GET /api/match/vacancy", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(matchHandler.CandidatesForVacancy))))
	mux.Handle("GET /api/match/recommendations", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(matchHandler.RecommendationsForStudent))))

	mux.Handle("GET /api/search", http.HandlerFunc(searchHandler.GlobalSearch))
	mux.Handle("GET /api/search/users", authMiddleware(middleware.RequireRole(model.RoleAdmin, model.RoleRecruiter)(http.HandlerFunc(searchHandler.SearchUsers))))
	mux.Handle("GET /api/search/students", authMiddleware(middleware.RequireRole(model.RoleAdmin, model.RoleRecruiter)(http.HandlerFunc(searchHandler.SearchStudents))))
	mux.Handle("GET /api/students/{id}", authMiddleware(middleware.RequireRole(model.RoleAdmin, model.RoleRecruiter, model.RoleStudent)(http.HandlerFunc(searchHandler.GetStudentByID))))
	mux.Handle("GET /api/students/{id}/portfolio", authMiddleware(middleware.RequireRole(model.RoleAdmin, model.RoleRecruiter, model.RoleStudent)(http.HandlerFunc(portfolioHandler.GetStudentPortfolio))))
	mux.Handle("GET /api/users", authMiddleware(middleware.RequireRole(allRoles...)(http.HandlerFunc(profileHandler.GetUserByID))))

	mux.Handle("GET /api/notifications", authMiddleware(middleware.RequireRole(allRoles...)(http.HandlerFunc(notifHandler.List))))
	mux.Handle("PATCH /api/notifications/read", authMiddleware(middleware.RequireRole(allRoles...)(http.HandlerFunc(notifHandler.MarkRead))))
	mux.Handle("GET /api/notifications/preferences", authMiddleware(middleware.RequireRole(allRoles...)(http.HandlerFunc(notifHandler.Preferences))))
	mux.Handle("PATCH /api/notifications/preferences", authMiddleware(middleware.RequireRole(allRoles...)(http.HandlerFunc(notifHandler.Preferences))))
	mux.Handle("POST /api/notifications/push/subscribe", authMiddleware(middleware.RequireRole(allRoles...)(http.HandlerFunc(notifHandler.PushSubscribe))))

	mux.Handle("GET /api/conversations", authMiddleware(middleware.RequireRole(model.RoleStudent, model.RoleRecruiter)(http.HandlerFunc(msgHandler.ListConversations))))
	mux.Handle("GET /api/conversations/messages", authMiddleware(middleware.RequireRole(model.RoleStudent, model.RoleRecruiter)(http.HandlerFunc(msgHandler.ListMessages))))
	mux.Handle("POST /api/conversations/messages", authMiddleware(middleware.RequireRole(model.RoleStudent, model.RoleRecruiter)(http.HandlerFunc(msgHandler.SendMessage))))

	mux.Handle("GET /api/wallet/me", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(walletHandler.Me))))
	mux.Handle("GET /api/wallet/transactions", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(walletHandler.Transactions))))
	mux.Handle("POST /api/wallet/withdraw", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(walletHandler.Withdraw))))

	mux.Handle("GET /api/moderator/queue", authMiddleware(middleware.RequireRole(modRoles...)(http.HandlerFunc(modHandler.Queue))))
	mux.Handle("PATCH /api/moderator/vacancies", authMiddleware(middleware.RequireRole(modRoles...)(http.HandlerFunc(modHandler.ReviewVacancy))))
	mux.Handle("PATCH /api/moderator/hackathons", authMiddleware(middleware.RequireRole(modRoles...)(http.HandlerFunc(modHandler.ReviewHackathon))))
	mux.Handle("GET /api/moderator/disputes", authMiddleware(middleware.RequireRole(modRoles...)(http.HandlerFunc(freelanceHandler.ListDisputes))))

	mux.Handle("GET /api/admin/dashboard", authMiddleware(middleware.RequireRole(model.RoleAdmin)(http.HandlerFunc(adminHandler.Dashboard))))
	mux.Handle("GET /api/admin/users", authMiddleware(middleware.RequireRole(model.RoleAdmin)(http.HandlerFunc(adminHandler.ListUsers))))
	mux.Handle("PATCH /api/admin/users", authMiddleware(middleware.RequireRole(model.RoleAdmin)(http.HandlerFunc(adminHandler.PatchUser))))
	mux.Handle("GET /api/admin/audit-log", authMiddleware(middleware.RequireRole(model.RoleAdmin)(http.HandlerFunc(adminHandler.AuditLog))))
	mux.Handle("GET /api/admin/transactions", authMiddleware(middleware.RequireRole(model.RoleAdmin)(http.HandlerFunc(adminHandler.ListTransactions))))
	mux.Handle("PATCH /api/admin/tariffs", authMiddleware(middleware.RequireRole(model.RoleAdmin)(http.HandlerFunc(adminHandler.PatchTariff))))
	mux.Handle("GET /api/admin/verifications", authMiddleware(middleware.RequireRole(model.RoleAdmin)(http.HandlerFunc(verificationHandler.ListAdmin))))
	mux.Handle("PATCH /api/admin/verifications", authMiddleware(middleware.RequireRole(model.RoleAdmin)(http.HandlerFunc(verificationHandler.PatchAdmin))))
	mux.Handle("GET /api/admin/withdrawals", authMiddleware(middleware.RequireRole(model.RoleAdmin)(http.HandlerFunc(adminHandler.ListWithdrawals))))
	mux.Handle("PATCH /api/admin/withdrawals", authMiddleware(middleware.RequireRole(model.RoleAdmin)(http.HandlerFunc(adminHandler.ProcessWithdrawal))))

	mux.Handle("POST /api/ai/career-chat", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(aiHandler.CareerChat))))
	mux.Handle("POST /api/ai/cover-letter", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(aiHandler.CoverLetter))))
	mux.Handle("POST /api/ai/analyze-resume", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(aiHandler.AnalyzeResume))))
	mux.Handle("POST /api/ai/interview-prep", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(aiHandler.InterviewPrep))))
	mux.Handle("POST /api/ai/improve-vacancy", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(aiHandler.ImproveVacancy))))
	mux.Handle("POST /api/ai/recommendations", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(aiHandler.Recommendations))))
	mux.Handle("POST /api/ai/suggest-candidates", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(aiHandler.SuggestCandidates))))

	mux.Handle("GET /api/billing/plans", authMiddleware(middleware.RequireRole(model.RoleRecruiter, model.RoleAdmin)(http.HandlerFunc(billingHandler.ListPlans))))
	mux.Handle("GET /api/billing/me", authMiddleware(middleware.RequireRole(model.RoleRecruiter, model.RoleAdmin)(http.HandlerFunc(billingHandler.Me))))
	mux.Handle("POST /api/billing/subscribe", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(billingHandler.Subscribe))))
	mux.Handle("POST /api/billing/checkout", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(billingHandler.Checkout))))
	mux.Handle("POST /api/billing/apply-promo", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(billingHandler.ApplyPromo))))
	mux.Handle("GET /api/billing/payment-methods", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(billingHandler.ListPaymentMethods))))
	mux.Handle("POST /api/billing/payment-methods", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(billingHandler.AddPaymentMethod))))
	mux.Handle("DELETE /api/billing/payment-methods", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(billingHandler.DeletePaymentMethod))))
	mux.Handle("POST /api/billing/publish-vacancy", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(billingHandler.PublishVacancy))))
	mux.Handle("POST /api/billing/promote-vacancy", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(billingHandler.PromoteVacancy))))
	mux.Handle("POST /api/billing/publish-hackathon", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(billingHandler.PublishHackathon))))
	mux.Handle("GET /api/billing/analytics", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(billingHandler.Analytics))))

	mux.Handle("POST /api/freelance/tasks", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(freelanceHandler.CreateTask))))
	mux.Handle("GET /api/freelance/tasks", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("mine") == "true" {
			authMiddleware(middleware.RequireRole(model.RoleStudent, model.RoleRecruiter)(http.HandlerFunc(freelanceHandler.ListMine))).ServeHTTP(w, r)
			return
		}
		if r.URL.Query().Get("id") != "" {
			freelanceHandler.GetTask(w, r)
			return
		}
		freelanceHandler.ListTasks(w, r)
	}))
	mux.Handle("GET /api/freelance/proposals", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(freelanceHandler.ListProposals))))
	mux.Handle("POST /api/freelance/proposals", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(freelanceHandler.CreateProposal))))
	mux.Handle("PATCH /api/freelance/proposals", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(freelanceHandler.UpdateProposal))))
	mux.Handle("POST /api/freelance/submissions", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(freelanceHandler.CreateSubmission))))
	mux.Handle("PATCH /api/freelance/submissions", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(freelanceHandler.UpdateSubmission))))
	mux.Handle("POST /api/freelance/reviews", authMiddleware(middleware.RequireRole(model.RoleStudent, model.RoleRecruiter)(http.HandlerFunc(freelanceHandler.CreateReview))))
	mux.Handle("POST /api/freelance/tasks/complete", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(freelanceHandler.CompleteTask))))
	mux.Handle("POST /api/freelance/disputes", authMiddleware(middleware.RequireRole(model.RoleStudent, model.RoleRecruiter)(http.HandlerFunc(freelanceHandler.CreateDispute))))
	mux.Handle("PATCH /api/admin/freelance/disputes", authMiddleware(middleware.RequireRole(modRoles...)(http.HandlerFunc(freelanceHandler.ResolveDispute))))

	mux.Handle("POST /api/hackathons", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(hackathonHandler.Create))))
	mux.Handle("GET /api/hackathons", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("mine") == "true" {
			authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(hackathonHandler.ListMine))).ServeHTTP(w, r)
			return
		}
		if r.URL.Query().Get("leaderboard") == "true" {
			hackathonHandler.Leaderboard(w, r)
			return
		}
		if r.URL.Query().Get("announcements") == "true" {
			hackathonHandler.ListAnnouncements(w, r)
			return
		}
		if r.URL.Query().Get("criteria") == "true" {
			hackathonHandler.ListCriteria(w, r)
			return
		}
		if r.URL.Query().Get("id") != "" {
			hackathonHandler.Get(w, r)
			return
		}
		hackathonHandler.List(w, r)
	}))
	mux.Handle("POST /api/hackathons/publish", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(hackathonHandler.Publish))))
	mux.Handle("POST /api/hackathons/register", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(hackathonHandler.Register))))
	mux.Handle("POST /api/hackathons/teams", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(hackathonHandler.CreateTeam))))
	mux.Handle("GET /api/hackathons/team-requests", authMiddleware(middleware.RequireRole(allRoles...)(http.HandlerFunc(hackathonHandler.ListTeamRequests))))
	mux.Handle("POST /api/hackathons/team-requests", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(hackathonHandler.CreateTeamRequest))))
	mux.Handle("PATCH /api/hackathons/team-requests", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(hackathonHandler.PatchTeamRequest))))
	mux.Handle("POST /api/hackathons/criteria", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(hackathonHandler.CreateCriterion))))
	mux.Handle("PATCH /api/hackathons/criteria", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(hackathonHandler.UpdateCriterion))))
	mux.Handle("DELETE /api/hackathons/criteria", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(hackathonHandler.DeleteCriterion))))
	mux.Handle("POST /api/hackathons/scores", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(hackathonHandler.SubmitScores))))
	mux.Handle("POST /api/hackathons/submissions", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(hackathonHandler.Submit))))
	mux.Handle("POST /api/hackathons/results", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(hackathonHandler.PublishResults))))
	mux.Handle("POST /api/hackathons/announcements", authMiddleware(middleware.RequireRole(model.RoleRecruiter)(http.HandlerFunc(hackathonHandler.AddAnnouncement))))
	mux.Handle("GET /api/hackathons/certificates/me", authMiddleware(middleware.RequireRole(model.RoleStudent)(http.HandlerFunc(hackathonHandler.MyCertificates))))

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

	srv := &http.Server{Addr: ":" + cfg.Server.Port, Handler: middleware.Logging(corsHandler)}
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
