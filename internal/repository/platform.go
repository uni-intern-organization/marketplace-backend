package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

type AuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

func (r *AuditRepository) Log(ctx context.Context, actorID *uuid.UUID, action, entityType string, entityID *uuid.UUID, metadata map[string]interface{}) error {
	var meta []byte
	if metadata != nil {
		var err error
		meta, err = json.Marshal(metadata)
		if err != nil {
			return err
		}
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_log (actor_id, action, entity_type, entity_id, metadata)
		VALUES ($1, $2, $3, $4, $5)
	`, actorID, action, entityType, entityID, meta)
	return err
}

func (r *AuditRepository) List(ctx context.Context, limit, offset int) ([]model.AuditEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, actor_id, action, COALESCE(entity_type,''), entity_id, metadata, created_at
		FROM audit_log ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.AuditEntry
	for rows.Next() {
		var e model.AuditEntry
		if err := rows.Scan(&e.ID, &e.ActorID, &e.Action, &e.EntityType, &e.EntityID, &e.Metadata, &e.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

type NotificationRepository struct {
	pool *pgxpool.Pool
}

func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

func (r *NotificationRepository) Create(ctx context.Context, userID uuid.UUID, nType, title, body string, payload map[string]interface{}) (*model.Notification, error) {
	var payloadBytes []byte
	if payload != nil {
		var err error
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}
	var n model.Notification
	err := r.pool.QueryRow(ctx, `
		INSERT INTO notifications (user_id, type, title, body, payload)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, type, title, body, payload, read_at, created_at
	`, userID, nType, title, body, payloadBytes).Scan(
		&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.Payload, &n.ReadAt, &n.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (r *NotificationRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]model.Notification, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, type, title, body, payload, read_at, created_at
		FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.Payload, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, n)
	}
	return list, rows.Err()
}

func (r *NotificationRepository) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`, userID).Scan(&n)
	return n, err
}

func (r *NotificationRepository) MarkRead(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) error {
	if len(ids) == 0 {
		_, err := r.pool.Exec(ctx, `UPDATE notifications SET read_at = NOW() WHERE user_id = $1 AND read_at IS NULL`, userID)
		return err
	}
	_, err := r.pool.Exec(ctx, `UPDATE notifications SET read_at = NOW() WHERE user_id = $1 AND id = ANY($2)`, userID, ids)
	return err
}

func (r *NotificationRepository) GetPreferences(ctx context.Context, userID uuid.UUID) ([]model.NotificationPreference, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT user_id, notification_type, channel_in_app, channel_email, channel_push
		FROM notification_preferences WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]model.NotificationPreference, 0)
	for rows.Next() {
		var p model.NotificationPreference
		if err := rows.Scan(&p.UserID, &p.NotificationType, &p.ChannelInApp, &p.ChannelEmail, &p.ChannelPush); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *NotificationRepository) UpsertPreference(ctx context.Context, p model.NotificationPreference) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO notification_preferences (user_id, notification_type, channel_in_app, channel_email, channel_push)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, notification_type) DO UPDATE SET
			channel_in_app = EXCLUDED.channel_in_app,
			channel_email = EXCLUDED.channel_email,
			channel_push = EXCLUDED.channel_push
	`, p.UserID, p.NotificationType, p.ChannelInApp, p.ChannelEmail, p.ChannelPush)
	return err
}

type MessagingRepository struct {
	pool *pgxpool.Pool
}

func NewMessagingRepository(pool *pgxpool.Pool) *MessagingRepository {
	return &MessagingRepository{pool: pool}
}

func (r *MessagingRepository) FindOrCreateConversation(ctx context.Context, studentID, recruiterID uuid.UUID, contextType string, contextID uuid.UUID, contextTitle string) (*model.Conversation, error) {
	var c model.Conversation
	err := r.pool.QueryRow(ctx, `
		INSERT INTO conversations (student_id, recruiter_id, context_type, context_id, context_title)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (student_id, recruiter_id, context_type, context_id) DO UPDATE SET updated_at = NOW()
		RETURNING id, student_id, recruiter_id, context_type, context_id, context_title, created_at, updated_at
	`, studentID, recruiterID, contextType, contextID, contextTitle).Scan(
		&c.ID, &c.StudentID, &c.RecruiterID, &c.ContextType, &c.ContextID, &c.ContextTitle, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *MessagingRepository) ListConversations(ctx context.Context, userID uuid.UUID) ([]model.Conversation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, student_id, recruiter_id, context_type, context_id, context_title, created_at, updated_at
		FROM conversations WHERE student_id = $1 OR recruiter_id = $1
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Conversation
	for rows.Next() {
		var c model.Conversation
		if err := rows.Scan(&c.ID, &c.StudentID, &c.RecruiterID, &c.ContextType, &c.ContextID, &c.ContextTitle, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *MessagingRepository) GetConversationByContext(ctx context.Context, contextType string, contextID uuid.UUID) (*model.Conversation, error) {
	var c model.Conversation
	err := r.pool.QueryRow(ctx, `
		SELECT id, student_id, recruiter_id, context_type, context_id, context_title, created_at, updated_at
		FROM conversations WHERE context_type = $1 AND context_id = $2
		ORDER BY updated_at DESC LIMIT 1
	`, contextType, contextID).Scan(
		&c.ID, &c.StudentID, &c.RecruiterID, &c.ContextType, &c.ContextID, &c.ContextTitle, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *MessagingRepository) GetConversation(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, error) {
	var c model.Conversation
	err := r.pool.QueryRow(ctx, `
		SELECT id, student_id, recruiter_id, context_type, context_id, context_title, created_at, updated_at
		FROM conversations WHERE id = $1 AND (student_id = $2 OR recruiter_id = $2)
	`, id, userID).Scan(&c.ID, &c.StudentID, &c.RecruiterID, &c.ContextType, &c.ContextID, &c.ContextTitle, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *MessagingRepository) AddMessage(ctx context.Context, conversationID, senderID uuid.UUID, bodyEnc []byte, attachmentKey *string) (*model.Message, error) {
	var m model.Message
	err := r.pool.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, sender_id, body_enc, attachment_key)
		VALUES ($1, $2, $3, $4)
		RETURNING id, conversation_id, sender_id, body_enc, attachment_key, read_at, created_at
	`, conversationID, senderID, bodyEnc, attachmentKey).Scan(
		&m.ID, &m.ConversationID, &m.SenderID, &m.BodyEnc, &m.AttachmentKey, &m.ReadAt, &m.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	_, _ = r.pool.Exec(ctx, `UPDATE conversations SET updated_at = NOW() WHERE id = $1`, conversationID)
	return &m, nil
}

func (r *MessagingRepository) ListMessages(ctx context.Context, conversationID uuid.UUID, limit int) ([]model.Message, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, conversation_id, sender_id, body_enc, attachment_key, read_at, created_at
		FROM messages WHERE conversation_id = $1 ORDER BY created_at ASC LIMIT $2
	`, conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Message
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.BodyEnc, &m.AttachmentKey, &m.ReadAt, &m.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *MessagingRepository) MarkMessagesRead(ctx context.Context, conversationID, readerID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE messages SET read_at = NOW()
		WHERE conversation_id = $1 AND sender_id != $2 AND read_at IS NULL
	`, conversationID, readerID)
	return err
}

type WalletRepository struct {
	pool *pgxpool.Pool
}

func NewWalletRepository(pool *pgxpool.Pool) *WalletRepository {
	return &WalletRepository{pool: pool}
}

func (r *WalletRepository) GetBalance(ctx context.Context, userID uuid.UUID) (float64, error) {
	var bal float64
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(balance_kzt, 0) FROM wallets WHERE user_id = $1`, userID).Scan(&bal)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	return bal, err
}

func (r *WalletRepository) Credit(ctx context.Context, userID uuid.UUID, amount float64, txType, refType string, refID *uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		INSERT INTO wallets (user_id, balance_kzt) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET balance_kzt = wallets.balance_kzt + $2, updated_at = NOW()
	`, userID, amount)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO wallet_transactions (user_id, amount_kzt, type, reference_type, reference_id, status)
		VALUES ($1, $2, $3, $4, $5, 'completed')
	`, userID, amount, txType, refType, refID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *WalletRepository) Debit(ctx context.Context, userID uuid.UUID, amount float64, txType string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var bal float64
	err = tx.QueryRow(ctx, `SELECT balance_kzt FROM wallets WHERE user_id = $1 FOR UPDATE`, userID).Scan(&bal)
	if err == pgx.ErrNoRows || bal < amount {
		return pgx.ErrNoRows
	}
	_, err = tx.Exec(ctx, `UPDATE wallets SET balance_kzt = balance_kzt - $2, updated_at = NOW() WHERE user_id = $1`, userID, amount)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO wallet_transactions (user_id, amount_kzt, type, status)
		VALUES ($1, $2, $3, 'completed')
	`, userID, -amount, txType)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *WalletRepository) ListTransactions(ctx context.Context, userID uuid.UUID, limit int) ([]model.WalletTransaction, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, amount_kzt, type, COALESCE(reference_type,''), reference_id, status, created_at
		FROM wallet_transactions WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.WalletTransaction
	for rows.Next() {
		var t model.WalletTransaction
		if err := rows.Scan(&t.ID, &t.UserID, &t.AmountKZT, &t.Type, &t.ReferenceType, &t.ReferenceID, &t.Status, &t.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *WalletRepository) CreateWithdrawal(ctx context.Context, userID uuid.UUID, amount float64, cardLast4 string) (*model.WithdrawalRequest, error) {
	if err := r.Debit(ctx, userID, amount, "withdrawal"); err != nil {
		return nil, err
	}
	var w model.WithdrawalRequest
	err := r.pool.QueryRow(ctx, `
		INSERT INTO withdrawal_requests (user_id, amount_kzt, card_last4, status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id, user_id, amount_kzt, card_last4, status, processed_by, created_at, processed_at
	`, userID, amount, cardLast4).Scan(
		&w.ID, &w.UserID, &w.AmountKZT, &w.CardLast4, &w.Status, &w.ProcessedBy, &w.CreatedAt, &w.ProcessedAt,
	)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *WalletRepository) ListWithdrawals(ctx context.Context, status string, limit int) ([]model.WithdrawalRequest, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows pgx.Rows
	var err error
	if status != "" {
		rows, err = r.pool.Query(ctx, `
			SELECT id, user_id, amount_kzt, card_last4, status, processed_by, created_at, processed_at
			FROM withdrawal_requests WHERE status = $1 ORDER BY created_at DESC LIMIT $2
		`, status, limit)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT id, user_id, amount_kzt, card_last4, status, processed_by, created_at, processed_at
			FROM withdrawal_requests ORDER BY created_at DESC LIMIT $1
		`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.WithdrawalRequest
	for rows.Next() {
		var w model.WithdrawalRequest
		if err := rows.Scan(&w.ID, &w.UserID, &w.AmountKZT, &w.CardLast4, &w.Status, &w.ProcessedBy, &w.CreatedAt, &w.ProcessedAt); err != nil {
			return nil, err
		}
		list = append(list, w)
	}
	return list, rows.Err()
}

func (r *WalletRepository) ProcessWithdrawal(ctx context.Context, id uuid.UUID, adminID uuid.UUID, approve bool) error {
	status := "completed"
	if !approve {
		status = "rejected"
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE withdrawal_requests SET status = $2, processed_by = $3, processed_at = NOW()
		WHERE id = $1
	`, id, status, adminID)
	return err
}

type ModerationRepository struct {
	pool *pgxpool.Pool
}

func NewModerationRepository(pool *pgxpool.Pool) *ModerationRepository {
	return &ModerationRepository{pool: pool}
}

func (r *ModerationRepository) CreateReview(ctx context.Context, entityType string, entityID, moderatorID uuid.UUID, action, reason, comment string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO moderation_reviews (entity_type, entity_id, moderator_id, action, reason, comment)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, entityType, entityID, moderatorID, action, reason, comment)
	return err
}

func (r *ModerationRepository) ListPendingVacancies(ctx context.Context, limit int) ([]model.Vacancy, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+vacancySelectCols+`
		FROM vacancies WHERE status = 'pending_review'
		ORDER BY COALESCE(moderation_submitted_at, created_at) ASC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Vacancy
	for rows.Next() {
		v, err := scanVacancy(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	return list, rows.Err()
}

func (r *ModerationRepository) SetVacancyStatus(ctx context.Context, id uuid.UUID, status model.VacancyStatus, activate bool) error {
	if activate {
		expires := time.Now().Add(30 * 24 * time.Hour)
		_, err := r.pool.Exec(ctx, `
			UPDATE vacancies SET status = $2, published_at = NOW(), expires_at = $3, updated_at = NOW()
			WHERE id = $1
		`, id, string(status), expires)
		return err
	}
	_, err := r.pool.Exec(ctx, `UPDATE vacancies SET status = $2, updated_at = NOW() WHERE id = $1`, id, string(status))
	return err
}

func (r *ModerationRepository) ListPendingHackathons(ctx context.Context, limit int) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT id FROM hackathons WHERE status = 'pending_review' ORDER BY created_at ASC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *ModerationRepository) SetHackathonStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE hackathons SET status = $2, updated_at = NOW() WHERE id = $1`, id, status)
	return err
}

func (r *ModerationRepository) GetVerification(ctx context.Context, recruiterID uuid.UUID) (*model.RecruiterVerification, error) {
	var v model.RecruiterVerification
	err := r.pool.QueryRow(ctx, `
		SELECT id, recruiter_id, bin, document_key, status, reviewed_by, comment, created_at, updated_at
		FROM recruiter_verifications WHERE recruiter_id = $1
	`, recruiterID).Scan(&v.ID, &v.RecruiterID, &v.BIN, &v.DocumentKey, &v.Status, &v.ReviewedBy, &v.Comment, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *ModerationRepository) UpsertVerification(ctx context.Context, recruiterID uuid.UUID, bin string, docKey *string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO recruiter_verifications (recruiter_id, bin, document_key, status)
		VALUES ($1, $2, $3, 'pending')
		ON CONFLICT (recruiter_id) DO UPDATE SET bin = $2, document_key = COALESCE($3, recruiter_verifications.document_key), status = 'pending', updated_at = NOW()
	`, recruiterID, bin, docKey)
	return err
}

func (r *ModerationRepository) ApproveVerification(ctx context.Context, recruiterID, reviewerID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE recruiter_verifications SET status = 'approved', reviewed_by = $2, updated_at = NOW()
		WHERE recruiter_id = $1
	`, recruiterID, reviewerID)
	return err
}
