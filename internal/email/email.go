package email

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"

	"github.com/uni-intern-organization/marketplace-backend/config"
)

type Service struct {
	cfg *config.SMTPConfig
}

func NewService(cfg *config.SMTPConfig) *Service {
	return &Service{cfg: cfg}
}

func (s *Service) Send(to, subject, body string) error {
	if !s.cfg.Enabled {
		log.Printf("email (demo): to=%s subject=%s", to, subject)
		return nil
	}
	msg := strings.Join([]string{
		fmt.Sprintf("From: %s", s.cfg.From),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"",
		body,
	}, "\r\n")
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	auth := smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)
	return smtp.SendMail(addr, auth, s.cfg.From, []string{to}, []byte(msg))
}

func (s *Service) SendPasswordReset(to, resetURL string) error {
	body := fmt.Sprintf(`<p>Сброс пароля Steppy Marketplace</p><p><a href="%s">Нажмите здесь</a> для сброса пароля. Ссылка действует 1 час.</p>`, resetURL)
	return s.Send(to, "Сброс пароля Steppy", body)
}

func (s *Service) SendWelcome(to string) error {
	body := `<p>Добро пожаловать в Steppy Marketplace!</p><p>Заполните профиль, чтобы получать лучшие рекомендации.</p>`
	return s.Send(to, "Добро пожаловать в Steppy", body)
}

func (s *Service) SendNotification(to, title, bodyText string) error {
	body := fmt.Sprintf("<p><strong>%s</strong></p><p>%s</p>", title, bodyText)
	return s.Send(to, title, body)
}
