package store

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

var ErrNotFound = errors.New("not found")

type Store struct{ pool *pgxpool.Pool }

type User struct {
	ID        int64
	Nickname  string
	Token     string
	DeviceFP  string
	CreatedAt time.Time
}

type Question struct {
	ID      int64    `json:"id"`
	Content string   `json:"content"`
	Options []string `json:"options"`
	Answer  int      `json:"answer"`
	Score   int      `json:"score"`
	Sort    int      `json:"sort"`
	Active  bool     `json:"active"`
}

type Attempt struct {
	ID         int64
	UserID     int64
	QuestionID int64
	Selected   *int
	Correct    bool
	Score      int
	RequestID  string
	CreatedAt  time.Time
}

type UserStats struct {
	Correct int
	Count   int
	First   time.Time
	Last    time.Time
}

type Qrcode struct {
	ID       int64
	ImageURL string
	Active   bool
}

type Background struct {
	ID        int64     `json:"id"`
	FilePath  string    `json:"file_path"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

type Announcement struct {
	ID        int64
	Content   string
	Type      string
	CreatedAt time.Time
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	s := &Store{pool: pool}
	if err := s.seedQuestions(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("seed: %w", err)
	}
	return s, nil
}

func (s *Store) Close() { s.pool.Close() }

// ---------- users ----------

func (s *Store) CreateUser(ctx context.Context, nickname, token, deviceFP string) (*User, error) {
	u := &User{Nickname: nickname, Token: token, DeviceFP: deviceFP}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (nickname, token, device_fp) VALUES ($1,$2,$3) RETURNING id, created_at`,
		nickname, token, deviceFP).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Store) GetUserByToken(ctx context.Context, token string) (*User, error) {
	return s.getUser(ctx, `SELECT id, nickname, token, device_fp, created_at FROM users WHERE token = $1`, token)
}

func (s *Store) GetUser(ctx context.Context, id int64) (*User, error) {
	return s.getUser(ctx, `SELECT id, nickname, token, device_fp, created_at FROM users WHERE id = $1`, id)
}

func (s *Store) getUser(ctx context.Context, q string, arg any) (*User, error) {
	u := &User{}
	err := s.pool.QueryRow(ctx, q, arg).Scan(&u.ID, &u.Nickname, &u.Token, &u.DeviceFP, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

// ---------- questions ----------

func (s *Store) ListActiveQuestions(ctx context.Context) ([]Question, error) {
	return s.listQuestions(ctx, `SELECT id, content, options, answer, score, sort, active FROM questions WHERE active ORDER BY sort, id`)
}

func (s *Store) AllQuestions(ctx context.Context) ([]Question, error) {
	return s.listQuestions(ctx, `SELECT id, content, options, answer, score, sort, active FROM questions ORDER BY sort, id`)
}

func (s *Store) listQuestions(ctx context.Context, q string) ([]Question, error) {
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Question{}
	for rows.Next() {
		var item Question
		var raw []byte
		if err := rows.Scan(&item.ID, &item.Content, &raw, &item.Answer, &item.Score, &item.Sort, &item.Active); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &item.Options); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetQuestion(ctx context.Context, id int64) (*Question, error) {
	item := &Question{}
	var raw []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, content, options, answer, score, sort, active FROM questions WHERE id = $1`, id).
		Scan(&item.ID, &item.Content, &raw, &item.Answer, &item.Score, &item.Sort, &item.Active)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(raw, &item.Options); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Store) CreateQuestion(ctx context.Context, content string, options []string, answer, score, sort int) (*Question, error) {
	raw, _ := json.Marshal(options)
	item := &Question{Content: content, Options: options, Answer: answer, Score: score, Sort: sort}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO questions (content, options, answer, score, sort) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		content, raw, answer, score, sort).Scan(&item.ID)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Store) SetQuestionActive(ctx context.Context, id int64, active bool) error {
	tag, err := s.pool.Exec(ctx, `UPDATE questions SET active = $2 WHERE id = $1`, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- attempts ----------

func (s *Store) InsertAttempt(ctx context.Context, a *Attempt) (*Attempt, bool, error) {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO attempts (user_id, question_id, selected, correct, score, request_id)
		 VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (request_id) DO NOTHING`,
		a.UserID, a.QuestionID, a.Selected, a.Correct, a.Score, a.RequestID)
	if err != nil {
		return nil, false, err
	}
	ex, err := s.GetAttemptByRequest(ctx, a.RequestID)
	if err != nil {
		return nil, false, err
	}
	return ex, tag.RowsAffected() == 0, nil
}

func (s *Store) GetAttemptByRequest(ctx context.Context, requestID string) (*Attempt, error) {
	a := &Attempt{}
	var sel int
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, question_id, COALESCE(selected,-1), correct, score, request_id, created_at
		 FROM attempts WHERE request_id = $1`, requestID).
		Scan(&a.ID, &a.UserID, &a.QuestionID, &sel, &a.Correct, &a.Score, &a.RequestID, &a.CreatedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if sel >= 0 {
		a.Selected = &sel
	}
	return a, nil
}

func (s *Store) UserStats(ctx context.Context, userID int64) (*UserStats, error) {
	st := &UserStats{}
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(CASE WHEN correct THEN 1 ELSE 0 END),0), COUNT(*),
		        COALESCE(MIN(created_at), now()), COALESCE(MAX(created_at), now())
		 FROM attempts WHERE user_id = $1`, userID).
		Scan(&st.Correct, &st.Count, &st.First, &st.Last)
	if err != nil {
		return nil, err
	}
	return st, nil
}

// ---------- quiz state ----------

func (s *Store) QuizState(ctx context.Context) (string, error) {
	var state string
	err := s.pool.QueryRow(ctx, `SELECT state FROM quiz_state WHERE id = 1`).Scan(&state)
	if err != nil {
		return "stop", err
	}
	return state, nil
}

func (s *Store) SetQuizState(ctx context.Context, state string) error {
	_, err := s.pool.Exec(ctx, `UPDATE quiz_state SET state = $1 WHERE id = 1`, state)
	return err
}

// ---------- qrcode ----------

func (s *Store) GetActiveQrcode(ctx context.Context) (*Qrcode, error) {
	q := &Qrcode{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, image_url, active FROM qrcodes WHERE active ORDER BY id DESC LIMIT 1`).
		Scan(&q.ID, &q.ImageURL, &q.Active)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return q, nil
}

func (s *Store) SetActiveQrcode(ctx context.Context, imageURL string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE qrcodes SET active = FALSE WHERE active`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO qrcodes (image_url, active) VALUES ($1, TRUE)`, imageURL); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ---------- announcements ----------

func (s *Store) InsertAnnouncement(ctx context.Context, content, typ string) (*Announcement, error) {
	a := &Announcement{Content: content, Type: typ}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO announcements (content, type) VALUES ($1,$2) RETURNING id, created_at`,
		content, typ).Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// ---------- backgrounds ----------

func (s *Store) CreateBackground(ctx context.Context, filePath string) (*Background, error) {
	b := &Background{FilePath: filePath, Active: true}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO backgrounds (file_path) VALUES ($1) RETURNING id, active, created_at`,
		filePath).Scan(&b.ID, &b.Active, &b.CreatedAt)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (s *Store) ListBackgrounds(ctx context.Context) ([]Background, error) {
	return s.listBackgrounds(ctx, `SELECT id, file_path, active, created_at FROM backgrounds ORDER BY id DESC`)
}

func (s *Store) ListActiveBackgrounds(ctx context.Context) ([]Background, error) {
	return s.listBackgrounds(ctx, `SELECT id, file_path, active, created_at FROM backgrounds WHERE active ORDER BY id DESC`)
}

func (s *Store) listBackgrounds(ctx context.Context, q string) ([]Background, error) {
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Background{}
	for rows.Next() {
		var b Background
		if err := rows.Scan(&b.ID, &b.FilePath, &b.Active, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) SetBackgroundActive(ctx context.Context, id int64, active bool) error {
	tag, err := s.pool.Exec(ctx, `UPDATE backgrounds SET active = $2 WHERE id = $1`, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteBackground 删除记录并返回文件路径，供调用方清理磁盘文件。
func (s *Store) DeleteBackground(ctx context.Context, id int64) (string, error) {
	var filePath string
	err := s.pool.QueryRow(ctx,
		`DELETE FROM backgrounds WHERE id = $1 RETURNING file_path`, id).Scan(&filePath)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return "", ErrNotFound
		}
		return "", err
	}
	return filePath, nil
}

func (s *Store) LatestAnnouncement(ctx context.Context) (*Announcement, error) {
	a := &Announcement{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, content, type, created_at FROM announcements ORDER BY id DESC LIMIT 1`).
		Scan(&a.ID, &a.Content, &a.Type, &a.CreatedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return a, nil
}
