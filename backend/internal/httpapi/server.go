package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"stpt/quiz/internal/config"
	"stpt/quiz/internal/quiz"
	"stpt/quiz/internal/store"
	"stpt/quiz/internal/ws"
)

type Server struct {
	cfg config.Config
	svc *quiz.Service
	hub *ws.Hub
}

func New(cfg config.Config, svc *quiz.Service, hub *ws.Hub) *Server {
	return &Server{cfg: cfg, svc: svc, hub: hub}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// 公开
	mux.HandleFunc("POST /api/session", s.handleSession)
	mux.HandleFunc("GET /api/rank", s.handleRank)
	mux.HandleFunc("GET /api/qrcode", s.handleQrcode)
	mux.HandleFunc("GET /api/config", s.handleConfig)

	// 需 token
	mux.HandleFunc("GET /api/questions", s.withUser(s.handleQuestions))
	mux.HandleFunc("POST /api/answer", s.withUser(s.handleAnswer))
	mux.HandleFunc("GET /api/me", s.withUser(s.handleMe))

	// WebSocket（可匿名=大屏）
	mux.HandleFunc("GET /ws", ws.Serve(s.hub, s.validateToken))

	// 后台（X-Admin-Token）
	mux.HandleFunc("POST /api/admin/quiz-state", s.withAdmin(s.handleAdminState))
	mux.HandleFunc("POST /api/admin/announce", s.withAdmin(s.handleAdminAnnounce))
	mux.HandleFunc("POST /api/admin/effect", s.withAdmin(s.handleAdminEffect))
	mux.HandleFunc("POST /api/admin/qrcode", s.withAdmin(s.handleAdminQrcode))
	mux.HandleFunc("GET /api/admin/questions", s.withAdmin(s.handleAdminListQuestions))
	mux.HandleFunc("POST /api/admin/questions", s.withAdmin(s.handleAdminCreateQuestion))
	mux.HandleFunc("POST /api/admin/questions/{id}/active", s.withAdmin(s.handleAdminSetActive))

	return withCORS(mux)
}

func (s *Server) validateToken(token string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := s.svc.ValidateToken(ctx, token)
	return err == nil
}

// ---------- middleware ----------

type userHandler func(http.ResponseWriter, *http.Request, *store.User)

func (s *Server) withUser(next userHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := r.URL.Query().Get("token")
		if tok == "" {
			if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
				tok = strings.TrimPrefix(h, "Bearer ")
			}
		}
		if tok == "" {
			writeErr(w, http.StatusUnauthorized, "缺少 token")
			return
		}
		u, err := s.svc.ValidateToken(r.Context(), tok)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "token 无效")
			return
		}
		next(w, r, u)
	}
}

func (s *Server) withAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Admin-Token")
		want := s.cfg.AdminPassword
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			writeErr(w, http.StatusUnauthorized, "后台口令错误")
			return
		}
		next(w, r)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	return dec.Decode(v)
}

// ---------- handlers ----------

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Nickname string `json:"nickname"`
		DeviceFP string `json:"device_fp"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体格式错误")
		return
	}
	u, err := s.svc.StartSession(r.Context(), req.Nickname, req.DeviceFP)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":  u.ID,
		"token":    u.Token,
		"nickname": u.Nickname,
	})
}

func (s *Server) handleQuestions(w http.ResponseWriter, r *http.Request, u *store.User) {
	qs, err := s.svc.Questions(r.Context(), u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"questions":        qs,
		"question_seconds": s.cfg.QuestionSeconds,
	})
}

func (s *Server) handleAnswer(w http.ResponseWriter, r *http.Request, u *store.User) {
	var req struct {
		RequestID  string `json:"request_id"`
		QuestionID int64  `json:"question_id"`
		Selected   *int   `json:"selected"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体格式错误")
		return
	}
	att, correctIdx, duplicate, err := s.svc.Submit(r.Context(), u.ID, req.RequestID, req.QuestionID, req.Selected)
	switch {
	case errors.Is(err, quiz.ErrQuizNotStarted):
		writeErr(w, http.StatusConflict, err.Error())
		return
	case errors.Is(err, quiz.ErrQuestionClosed):
		writeErr(w, http.StatusGone, err.Error())
		return
	case errors.Is(err, quiz.ErrBadRequest):
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, http.StatusNotFound, "题目不存在")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"correct":        att.Correct,
		"correct_answer": correctIdx,
		"score":          att.Score,
		"request_id":     att.RequestID,
		"duplicate":      duplicate,
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, u *store.User) {
	st, err := s.svc.MyStats(r.Context(), u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleRank(w http.ResponseWriter, r *http.Request) {
	n := int64(50)
	if v := r.URL.Query().Get("top"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed > 0 && parsed <= 200 {
			n = parsed
		}
	}
	entries, err := s.svc.Top(r.Context(), n)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Server) handleQrcode(w http.ResponseWriter, r *http.Request) {
	q, err := s.svc.Qrcode(r.Context())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"image_url": "", "active": false})
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"image_url": q.ImageURL, "active": q.Active})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	state, err := s.svc.State(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	announce, err := s.svc.LatestAnnouncement(r.Context())
	if err != nil {
		announce = ""
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"title":            s.svc.Title(),
		"quiz_state":       state,
		"question_seconds": s.cfg.QuestionSeconds,
		"announcement":     announce,
	})
}

// ---------- admin handlers ----------

func (s *Server) handleAdminState(w http.ResponseWriter, r *http.Request) {
	var req struct {
		State string `json:"state"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体格式错误")
		return
	}
	if err := s.svc.SetState(r.Context(), req.State); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true", "state": req.State})
}

func (s *Server) handleAdminAnnounce(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
		Type    string `json:"type"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体格式错误")
		return
	}
	if err := s.svc.Announce(r.Context(), req.Content, req.Type); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) handleAdminEffect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind string `json:"kind"`
	}
	_ = readJSON(r, &req)
	if err := s.svc.Effect(r.Context(), req.Kind); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true", "kind": req.Kind})
}

func (s *Server) handleAdminQrcode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ImageURL string `json:"image_url"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体格式错误")
		return
	}
	if err := s.svc.SetQrcode(r.Context(), req.ImageURL); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) handleAdminListQuestions(w http.ResponseWriter, r *http.Request) {
	qs, err := s.svc.AllQuestions(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"questions": qs})
}

func (s *Server) handleAdminCreateQuestion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string   `json:"content"`
		Options []string `json:"options"`
		Answer  int      `json:"answer"`
		Score   int      `json:"score"`
		Sort    int      `json:"sort"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体格式错误")
		return
	}
	q, err := s.svc.CreateQuestion(r.Context(), req.Content, req.Options, req.Answer, req.Score, req.Sort)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, q)
}

func (s *Server) handleAdminSetActive(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "id 无效")
		return
	}
	var req struct {
		Active bool `json:"active"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体格式错误")
		return
	}
	if err := s.svc.SetQuestionActive(r.Context(), id, req.Active); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "active": req.Active})
}
