package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"stpt/quiz/internal/config"
	"stpt/quiz/internal/quiz"
	"stpt/quiz/internal/store"
	"stpt/quiz/internal/ws"
)

type Server struct {
	cfg       config.Config
	svc       *quiz.Service
	hub       *ws.Hub
	startedAt time.Time
}

func New(cfg config.Config, svc *quiz.Service, hub *ws.Hub) *Server {
	return &Server{cfg: cfg, svc: svc, hub: hub, startedAt: time.Now()}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// 公开
	mux.HandleFunc("GET /api/backgrounds", s.handleBackgrounds)
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.HandleFunc("POST /api/identity/abandon", s.handleAbandonIdentity)
	mux.HandleFunc("GET /api/identity/current", s.handleCurrentIdentity)
	mux.HandleFunc("POST /api/session", s.handleSession)
	mux.HandleFunc("GET /api/rank", s.handleRank)
	mux.HandleFunc("GET /api/qrcode", s.handleQrcode)
	mux.HandleFunc("GET /api/config", s.handleConfig)

	// 需 token
	mux.HandleFunc("POST /api/quiz/round", s.withUser(s.handleStartRound))
	mux.HandleFunc("GET /api/quiz/current", s.withUser(s.handleCurrentRound))
	mux.HandleFunc("POST /api/answer", s.withUser(s.handleAnswer))
	mux.HandleFunc("GET /api/me", s.withUser(s.handleMe))

	// WebSocket（可匿名=大屏）
	mux.HandleFunc("GET /ws", ws.Serve(s.hub, s.validateToken))

	// 后台（X-Admin-Token）
	mux.HandleFunc("POST /api/admin/quiz-state", s.withAdmin(s.handleAdminState))
	mux.HandleFunc("GET /api/admin/overview", s.withAdmin(s.handleAdminOverview))
	mux.HandleFunc("GET /api/admin/export/scores", s.withAdmin(s.handleAdminExportScores))
	mux.HandleFunc("POST /api/admin/questions/import", s.withAdmin(s.handleAdminImportQuestions))
	mux.HandleFunc("GET /api/admin/questions/template", s.withAdmin(s.handleAdminQuestionsTemplate))
	mux.HandleFunc("POST /api/admin/announce", s.withAdmin(s.handleAdminAnnounce))
	mux.HandleFunc("POST /api/admin/effect", s.withAdmin(s.handleAdminEffect))
	mux.HandleFunc("POST /api/admin/qrcode", s.withAdmin(s.handleAdminQrcode))
	mux.HandleFunc("GET /api/admin/questions", s.withAdmin(s.handleAdminListQuestions))
	mux.HandleFunc("POST /api/admin/questions", s.withAdmin(s.handleAdminCreateQuestion))
	mux.HandleFunc("POST /api/admin/questions/{id}/active", s.withAdmin(s.handleAdminSetActive))
	mux.HandleFunc("POST /api/admin/backgrounds", s.withAdmin(s.handleAdminUploadBackground))
	mux.HandleFunc("GET /api/admin/backgrounds", s.withAdmin(s.handleAdminListBackgrounds))
	mux.HandleFunc("POST /api/admin/backgrounds/{id}/active", s.withAdmin(s.handleAdminSetBgActive))
	mux.HandleFunc("DELETE /api/admin/backgrounds/{id}", s.withAdmin(s.handleAdminDeleteBackground))
	mux.HandleFunc("POST /api/admin/uploads", s.withAdmin(s.handleAdminUpload))
	mux.HandleFunc("GET /api/admin/footer", s.withAdmin(s.handleAdminGetFooter))
	mux.HandleFunc("POST /api/admin/footer", s.withAdmin(s.handleAdminSaveFooter))

	// 上传文件静态服务（生产由 nginx 直出共享卷，此处兜底开发模式）
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(s.cfg.UploadDir))))

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

// handleCurrentIdentity 查询本机当前绑定的答题身份。
func (s *Server) handleCurrentIdentity(w http.ResponseWriter, r *http.Request) {
	fp := r.URL.Query().Get("device_fp")
	if fp == "" {
		writeErr(w, http.StatusBadRequest, "缺少 device_fp")
		return
	}
	u, err := s.svc.BoundIdentity(r.Context(), fp)
	if errors.Is(err, store.ErrNotFound) || u == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "未绑定身份"})
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"nickname": u.Nickname,
		"user_id":  u.ID,
	})
}

// handleAbandonIdentity 放弃答题身份：
// 带 token 时必须同时提供本机 device_fp 且与身份绑定一致（不能放弃他人身份）；
// 否则按 device_fp 放弃本机绑定身份。
func (s *Server) handleAbandonIdentity(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceFP string `json:"device_fp"`
	}
	_ = readJSON(r, &req)

	tok := r.URL.Query().Get("token")
	if tok == "" {
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			tok = strings.TrimPrefix(h, "Bearer ")
		}
	}
	if tok != "" {
		u, err := s.svc.ValidateToken(r.Context(), tok)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "token 无效")
			return
		}
		if err := s.svc.Abandon(r.Context(), u.ID, req.DeviceFP); err != nil {
			writeErr(w, http.StatusForbidden, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		return
	}
	if req.DeviceFP == "" {
		writeErr(w, http.StatusBadRequest, "缺少 token 或 device_fp")
		return
	}
	if err := s.svc.AbandonByDeviceFP(r.Context(), req.DeviceFP); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.svc.Stats(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, online := s.hub.ClientCounts()
	writeJSON(w, http.StatusOK, map[string]any{
		"participants": st.Participants,
		"attempts":     st.Attempts,
		"online":       online,
	})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Nickname string `json:"nickname"`
		DeviceFP string `json:"device_fp"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体格式错误")
		return
	}
	u, recovered, err := s.svc.StartSession(r.Context(), req.Nickname, req.DeviceFP)
	if errors.Is(err, quiz.ErrIdentityBound) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error(), "code": "identity_bound"})
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":   u.ID,
		"token":     u.Token,
		"nickname":  u.Nickname,
		"recovered": recovered,
	})
}

func (s *Server) quizLimit() int {
	if s.cfg.QuizCount <= 0 {
		return 10
	}
	return s.cfg.QuizCount
}

func (s *Server) handleStartRound(w http.ResponseWriter, r *http.Request, u *store.User) {
	var req struct {
		Difficulty string `json:"difficulty"`
	}
	_ = readJSON(r, &req)
	if req.Difficulty == "" {
		req.Difficulty = "全部"
	}
	roundNo, qs, err := s.svc.StartRound(r.Context(), u.ID, req.Difficulty, s.quizLimit())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"round_no":         roundNo,
		"questions":        qs,
		"question_seconds": s.cfg.QuestionSeconds,
	})
}

func (s *Server) handleCurrentRound(w http.ResponseWriter, r *http.Request, u *store.User) {
	roundNo, qs, answered, err := s.svc.CurrentRound(r.Context(), u.ID, s.quizLimit())
	if errors.Is(err, quiz.ErrNoRound) {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"round_no":         roundNo,
		"questions":        qs,
		"question_seconds": s.cfg.QuestionSeconds,
		"answered":         answered,
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
	case errors.Is(err, quiz.ErrNoRound):
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
	t0 := time.Now()
	defer func() {
		if d := time.Since(t0); d > 80*time.Millisecond {
			log.Printf("rank slow: %v", d)
		}
	}()
	n := 99
	if v := r.URL.Query().Get("top"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 500 {
			n = parsed
		}
	}
	entries, total, err := s.svc.Leaderboard(r.Context(), n)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "total": total})
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
	footer, err := s.svc.Footer(r.Context())
	if err != nil {
		footer = &store.Footer{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"title":            s.svc.Title(),
		"quiz_state":       state,
		"question_seconds": s.cfg.QuestionSeconds,
		"announcement":     announce,
		"footer":           footer,
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
	old, _ := s.svc.Qrcode(r.Context())
	if err := s.svc.SetQrcode(r.Context(), req.ImageURL); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if old != nil {
		s.removeUploadFile(old.ImageURL)
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
		Content    string   `json:"content"`
		Options    []string `json:"options"`
		Answer     int      `json:"answer"`
		Score      int      `json:"score"`
		Sort       int      `json:"sort"`
		Difficulty string   `json:"difficulty"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体格式错误")
		return
	}
	q, err := s.svc.CreateQuestion(r.Context(), req.Content, req.Options, req.Answer, req.Score, req.Sort, req.Difficulty)
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

// csvSafe 防止 Excel/表格软件的公式注入（= + - @ 等开头加单引号转义）。
func csvSafe(v string) string {
	if v == "" {
		return v
	}
	switch v[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + v
	}
	return v
}

// ---------- admin: 导出成绩 CSV ----------

func (s *Server) handleAdminExportScores(w http.ResponseWriter, r *http.Request) {
	rows, err := s.svc.ExportScores(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM，Excel 中文兼容
	cw := csv.NewWriter(&buf)
	_ = cw.Write([]string{"排名", "昵称", "答对", "已答", "得分", "用时(秒)", "轮数", "状态", "注册时间"})
	for _, row := range rows {
		rankStr := "-"
		if row.Rank > 0 {
			rankStr = strconv.FormatInt(row.Rank, 10)
		}
		status := "活跃"
		if row.Abandoned {
			status = "已放弃"
		}
		_ = cw.Write([]string{
			rankStr, csvSafe(row.Nickname),
			strconv.Itoa(row.Correct), strconv.Itoa(row.Answered), strconv.Itoa(row.Score),
			fmt.Sprintf("%.1f", float64(row.ElapsedMS)/1000), strconv.Itoa(row.Rounds),
			status, row.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	cw.Flush()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="scores-%s.csv"`, time.Now().Format("20060102-150405")))
	_, _ = w.Write(buf.Bytes())
}

// ---------- admin: 批量导入题目 CSV ----------

func (s *Server) handleAdminQuestionsTemplate(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	cw := csv.NewWriter(&buf)
	_ = cw.Write([]string{"题目", "选项A", "选项B", "选项C", "选项D", "选项E", "选项F", "正确答案", "难度", "分值"})
	_ = cw.Write([]string{"示例：二进制 1010 对应的十进制是？", "8", "9", "10", "12", "", "", "C", "容易", "10"})
	cw.Flush()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="questions-template.csv"`)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) handleAdminImportQuestions(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "文件过大或格式错误（上限 5MB）")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "缺少 file 字段")
		return
	}
	defer file.Close()
	recs, err := csv.NewReader(file).ReadAll()
	if err != nil {
		writeErr(w, http.StatusBadRequest, "CSV 解析失败："+err.Error())
		return
	}

	imported := 0
	failed := []map[string]any{}
	for i, rec := range recs {
		line := i + 1
		if i == 0 && strings.Contains(rec[0], "题目") {
			continue
		}
		content := strings.TrimSpace(rec[0])
		if content == "" || strings.HasPrefix(content, "示例") {
			continue
		}
		var opts []string
		for _, o := range rec[1:7] {
			if strings.TrimSpace(o) != "" {
				opts = append(opts, strings.TrimSpace(o))
			}
		}
		if len(opts) > 0 && len(rec) > 7 {
			// 兼容「正确答案」放在末列的情况
		}
		ansCol := ""
		difficulty := "简单"
		score := 10
		if len(rec) > 7 {
			ansCol = strings.TrimSpace(rec[7])
		} else if len(opts) > 0 {
			ansCol = strings.TrimSpace(rec[len(rec)-1])
		}
		if len(rec) > 8 {
			if d := strings.TrimSpace(rec[8]); d != "" {
				difficulty = d
			}
		}
		if len(rec) > 9 {
			if v, err := strconv.Atoi(strings.TrimSpace(rec[9])); err == nil && v > 0 {
				score = v
			}
		}
		answerIdx := -1
		if u := strings.ToUpper(ansCol); len(u) == 1 && u[0] >= 'A' && u[0] <= 'F' {
			answerIdx = int(u[0] - 'A')
		} else if n, err := strconv.Atoi(ansCol); err == nil {
			answerIdx = n
		}
		_, cerr := s.svc.CreateQuestion(r.Context(), content, opts, answerIdx, score, 0, difficulty)
		if cerr != nil {
			failed = append(failed, map[string]any{"line": line, "error": cerr.Error()})
			continue
		}
		imported++
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": imported, "failed": failed})
}

// ---------- admin: overview（资源与访问情况） ----------

func (s *Server) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	_, online := s.hub.ClientCounts()
	ov, err := s.svc.Overview(r.Context(), int64(online))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	writeJSON(w, http.StatusOK, map[string]any{
		"overview":    ov,
		"uptime":      time.Since(s.startedAt).Round(time.Second).String(),
		"goroutines":  runtime.NumGoroutine(),
		"mem_mb":      int(m.Alloc / 1024 / 1024),
		"app_version": "1.0",
	})
}

// ---------- admin: backgrounds ----------

func (s *Server) handleBackgrounds(w http.ResponseWriter, r *http.Request) {
	bgs, err := s.svc.Backgrounds(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	urls := make([]string, 0, len(bgs))
	for _, b := range bgs {
		urls = append(urls, b.FilePath)
	}
	writeJSON(w, http.StatusOK, map[string]any{"backgrounds": urls})
}

// saveUploadedImage 保存 multipart 里的图片文件，返回 /uploads/xxx 路径；失败时已写响应。
func (s *Server) saveUploadedImage(w http.ResponseWriter, r *http.Request) (string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "文件过大或格式错误（上限 10MB）")
		return "", false
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "缺少 file 字段")
		return "", false
	}
	defer file.Close()

	head := make([]byte, 512)
	n, _ := file.Read(head)
	ct := http.DetectContentType(head[:n])
	ext := ""
	switch ct {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	case "image/gif":
		ext = ".gif"
	default:
		writeErr(w, http.StatusBadRequest, "仅支持 JPG/PNG/WebP/GIF 图片")
		return "", false
	}
	if _, err := file.Seek(0, 0); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return "", false
	}
	if err := os.MkdirAll(s.cfg.UploadDir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return "", false
	}
	name := fmt.Sprintf("up-%d-%s%s", time.Now().UnixNano(), randHex(4), ext)
	out, err := os.Create(filepath.Join(s.cfg.UploadDir, name))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return "", false
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		writeErr(w, http.StatusInternalServerError, err.Error())
		return "", false
	}
	out.Close()
	return "/uploads/" + name, true
}

// removeUploadFile 清理 /uploads/ 下的旧文件（切换后防止残留）。
func (s *Server) removeUploadFile(p string) {
	if p == "" || !strings.HasPrefix(p, "/uploads/") {
		return
	}
	_ = os.Remove(filepath.Join(s.cfg.UploadDir, filepath.Base(p)))
}

// 通用上传：返回文件路径，供二维码/备案图标等使用
func (s *Server) handleAdminUpload(w http.ResponseWriter, r *http.Request) {
	url, ok := s.saveUploadedImage(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

func (s *Server) handleAdminUploadBackground(w http.ResponseWriter, r *http.Request) {
	url, ok := s.saveUploadedImage(w, r)
	if !ok {
		return
	}
	bg, err := s.svc.AddBackground(r.Context(), url)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bg)
}

func (s *Server) handleAdminListBackgrounds(w http.ResponseWriter, r *http.Request) {
	bgs, err := s.svc.AllBackgrounds(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backgrounds": bgs})
}

func (s *Server) handleAdminSetBgActive(w http.ResponseWriter, r *http.Request) {
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
	if err := s.svc.SetBackgroundActive(r.Context(), id, req.Active); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "active": req.Active})
}

func (s *Server) handleAdminDeleteBackground(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "id 无效")
		return
	}
	filePath, err := s.svc.DeleteBackground(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = os.Remove(filepath.Join(s.cfg.UploadDir, filepath.Base(filePath)))
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "ok": "true"})
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------- admin: footer ----------

func (s *Server) handleAdminGetFooter(w http.ResponseWriter, r *http.Request) {
	f, err := s.svc.Footer(r.Context())
	if err != nil {
		f = &store.Footer{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"footer": f})
}

func (s *Server) handleAdminSaveFooter(w http.ResponseWriter, r *http.Request) {
	var req store.Footer
	if err := readJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体格式错误")
		return
	}
	old, _ := s.svc.Footer(r.Context())
	if err := s.svc.SaveFooter(r.Context(), &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	// 图标被替换/清空后清理旧文件
	if old != nil {
		if old.ICPIcon != req.ICPIcon {
			s.removeUploadFile(old.ICPIcon)
		}
		if old.PoliceIcon != req.PoliceIcon {
			s.removeUploadFile(old.PoliceIcon)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}
