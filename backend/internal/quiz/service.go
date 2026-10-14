package quiz

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	mathrand "math/rand"
	"strings"

	"stpt/quiz/internal/publish"
	"stpt/quiz/internal/rank"
	"stpt/quiz/internal/store"
)

var (
	ErrQuizNotStarted = errors.New("答题未开始")
	ErrQuestionClosed = errors.New("题目不可用")
	ErrBadRequest     = errors.New("参数错误")
)

type Service struct {
	store *store.Store
	rank  *rank.Rank
	pub   *publish.Pub
	title string
}

func New(st *store.Store, rk *rank.Rank, pub *publish.Pub, title string) *Service {
	return &Service{store: st, rank: rk, pub: pub, title: title}
}

func (s *Service) Title() string { return s.title }

// ---------- session ----------

func (s *Service) StartSession(ctx context.Context, nickname, deviceFP string) (*store.User, error) {
	nick := strings.TrimSpace(nickname)
	r := []rune(nick)
	if len(r) == 0 || len(r) > 16 {
		return nil, errors.New("昵称长度需为 1-16 个字符")
	}
	for _, c := range nick {
		if c < 0x20 || c == 0x7f {
			return nil, errors.New("昵称含非法字符")
		}
	}
	if containsBadWord(nick) {
		return nil, errors.New("昵称含敏感词，请更换")
	}
	token := randomHex(32)
	return s.store.CreateUser(ctx, nick, token, deviceFP)
}

func (s *Service) ValidateToken(ctx context.Context, token string) (*store.User, error) {
	if token == "" {
		return nil, ErrBadRequest
	}
	return s.store.GetUserByToken(ctx, token)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = cryptorand.Read(b)
	return hex.EncodeToString(b)
}

func containsBadWord(s string) bool {
	bad := []string{"傻逼", "傻比", "傻叉", "弱智", "智障", "脑残", "fuck", "shit", "nmsl", "cnm", "sb"}
	lower := strings.ToLower(s)
	for _, b := range bad {
		if strings.Contains(lower, b) {
			return true
		}
	}
	return false
}

// ---------- questions ----------

// orderedOptions 按用户维度确定性地打乱选项，提交判分时用同一顺序还原。
func orderedOptions(q store.Question, userID int64) []string {
	opts := append([]string(nil), q.Options...)
	r := mathrand.New(mathrand.NewSource(userID*104729 + int64(q.ID)*31 + 7))
	r.Shuffle(len(opts), func(i, j int) { opts[i], opts[j] = opts[j], opts[i] })
	return opts
}

// Questions 按用户确定性随机抽取 limit 题（同用户刷新结果一致，保证断线恢复；
// 不同用户抽到的题目不同；换身份重答也会换题）。
func (s *Service) Questions(ctx context.Context, userID int64, limit int) ([]store.Question, error) {
	qs, err := s.store.ListActiveQuestions(ctx)
	if err != nil {
		return nil, err
	}
	r := mathrand.New(mathrand.NewSource(userID*7919 + 13))
	r.Shuffle(len(qs), func(i, j int) { qs[i], qs[j] = qs[j], qs[i] })
	if limit > 0 && len(qs) > limit {
		qs = qs[:limit]
	}
	for i := range qs {
		qs[i].Options = orderedOptions(qs[i], userID)
	}
	return qs, nil
}

// correctIndex 返回该用户视角下正确答案的下标。
func correctIndex(q store.Question, userID int64) int {
	order := orderedOptions(q, userID)
	answer := q.Options[q.Answer]
	for i, o := range order {
		if o == answer {
			return i
		}
	}
	return -1
}

// ---------- answer ----------

func (s *Service) Submit(ctx context.Context, userID int64, requestID string, questionID int64, selected *int) (att *store.Attempt, correctIdx int, duplicate bool, err error) {
	if requestID == "" {
		return nil, -1, false, ErrBadRequest
	}
	state, err := s.store.QuizState(ctx)
	if err != nil {
		return nil, -1, false, err
	}
	if state != "start" {
		return nil, -1, false, ErrQuizNotStarted
	}
	q, err := s.store.GetQuestion(ctx, questionID)
	if err != nil {
		return nil, -1, false, err
	}
	if !q.Active {
		return nil, -1, false, ErrQuestionClosed
	}

	correct := false
	if selected != nil && *selected >= 0 {
		order := orderedOptions(*q, userID)
		if *selected < len(order) && order[*selected] == q.Options[q.Answer] {
			correct = true
		}
	}
	score := 0
	if correct {
		score = q.Score
	}
	att, duplicate, err = s.store.InsertAttempt(ctx, &store.Attempt{
		UserID:     userID,
		QuestionID: questionID,
		Selected:   selected,
		Correct:    correct,
		Score:      score,
		RequestID:  requestID,
	})
	if err != nil {
		return nil, -1, false, err
	}
	if !duplicate {
		s.refreshRank(ctx, userID)
	}
	return att, correctIndex(*q, userID), duplicate, nil
}

func (s *Service) refreshRank(ctx context.Context, userID int64) {
	stats, err := s.store.UserStats(ctx, userID)
	if err != nil {
		return
	}
	elapsed := int64(0)
	if stats.Count > 0 {
		elapsed = stats.Last.Sub(stats.First).Milliseconds()
	}
	u, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return
	}
	_ = s.rank.Update(ctx, userID, u.Nickname, stats.Correct, elapsed)
}

// ---------- stats & rank ----------

type MyStats struct {
	UserID    int64  `json:"user_id"`
	Nickname  string `json:"nickname"`
	Correct   int    `json:"correct"`
	Answered  int    `json:"answered"`
	ElapsedMS int64  `json:"elapsed_ms"`
	Score     int    `json:"score"`
	Rank      int64  `json:"rank"` // 0 起；-1 表示未上榜
	Total     int64  `json:"total"`
}

func (s *Service) MyStats(ctx context.Context, userID int64) (*MyStats, error) {
	u, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	st, err := s.store.UserStats(ctx, userID)
	if err != nil {
		return nil, err
	}
	elapsed := int64(0)
	if st.Count > 0 {
		elapsed = st.Last.Sub(st.First).Milliseconds()
	}
	rnk, err := s.rank.RankOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	total, err := s.rank.Count(ctx)
	if err != nil {
		return nil, err
	}
	return &MyStats{
		UserID:    u.ID,
		Nickname:  u.Nickname,
		Correct:   st.Correct,
		Answered:  st.Count,
		ElapsedMS: elapsed,
		Score:     st.Correct * 10,
		Rank:      rnk + 1, // 展示用 1 起；未上榜时 rnk=-1 → 0
		Total:     total,
	}, nil
}

func (s *Service) Top(ctx context.Context, n int64) ([]rank.Entry, error) {
	return s.rank.Top(ctx, n)
}

// ---------- footer ----------

func (s *Service) Footer(ctx context.Context) (*store.Footer, error) {
	return s.store.GetFooterSettings(ctx)
}

func (s *Service) SaveFooter(ctx context.Context, owner, license, icp string) error {
	owner = strings.TrimSpace(owner)
	license = strings.TrimSpace(license)
	icp = strings.TrimSpace(icp)
	if len([]rune(owner)) > 100 || len([]rune(license)) > 100 || len([]rune(icp)) > 50 {
		return ErrBadRequest
	}
	return s.store.SaveFooterSettings(ctx, owner, license, icp)
}

// ---------- backgrounds ----------

func (s *Service) Backgrounds(ctx context.Context) ([]store.Background, error) {
	return s.store.ListActiveBackgrounds(ctx)
}

func (s *Service) AllBackgrounds(ctx context.Context) ([]store.Background, error) {
	return s.store.ListBackgrounds(ctx)
}

func (s *Service) AddBackground(ctx context.Context, filePath string) (*store.Background, error) {
	return s.store.CreateBackground(ctx, filePath)
}

func (s *Service) SetBackgroundActive(ctx context.Context, id int64, active bool) error {
	return s.store.SetBackgroundActive(ctx, id, active)
}

// DeleteBackground 删除记录并返回文件路径，供调用方清理磁盘文件。
func (s *Service) DeleteBackground(ctx context.Context, id int64) (string, error) {
	return s.store.DeleteBackground(ctx, id)
}

// ---------- state / qrcode / announce / effect ----------

func (s *Service) State(ctx context.Context) (string, error) {
	return s.store.QuizState(ctx)
}

func (s *Service) SetState(ctx context.Context, state string) error {
	if state != "start" && state != "stop" {
		return ErrBadRequest
	}
	if err := s.store.SetQuizState(ctx, state); err != nil {
		return err
	}
	return s.publish(ctx, "quiz_state", map[string]any{"state": state})
}

func (s *Service) Qrcode(ctx context.Context) (*store.Qrcode, error) {
	return s.store.GetActiveQrcode(ctx)
}

func (s *Service) SetQrcode(ctx context.Context, imageURL string) error {
	if strings.TrimSpace(imageURL) == "" {
		return ErrBadRequest
	}
	return s.store.SetActiveQrcode(ctx, imageURL)
}

func (s *Service) Announce(ctx context.Context, content, typ string) error {
	content = strings.TrimSpace(content)
	if content == "" || len([]rune(content)) > 200 {
		return ErrBadRequest
	}
	if typ == "" {
		typ = "info"
	}
	if _, err := s.store.InsertAnnouncement(ctx, content, typ); err != nil {
		return err
	}
	return s.publish(ctx, "announcement", map[string]any{"content": content, "type": typ})
}

func (s *Service) LatestAnnouncement(ctx context.Context) (string, error) {
	a, err := s.store.LatestAnnouncement(ctx)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	return a.Content, nil
}

func (s *Service) Effect(ctx context.Context, kind string) error {
	if kind == "" {
		kind = "confetti"
	}
	return s.publish(ctx, "effect", map[string]any{"kind": kind})
}

func (s *Service) publish(ctx context.Context, typ string, data any) error {
	msg, err := json.Marshal(map[string]any{"type": typ, "data": data})
	if err != nil {
		return err
	}
	return s.pub.Publish(ctx, string(msg))
}

// ---------- admin: questions ----------

func (s *Service) AllQuestions(ctx context.Context) ([]store.Question, error) {
	return s.store.AllQuestions(ctx)
}

func (s *Service) CreateQuestion(ctx context.Context, content string, options []string, answer, score, sort int) (*store.Question, error) {
	content = strings.TrimSpace(content)
	if content == "" || len(options) < 2 || len(options) > 6 || answer < 0 || answer >= len(options) {
		return nil, ErrBadRequest
	}
	if score <= 0 {
		score = 10
	}
	return s.store.CreateQuestion(ctx, content, options, answer, score, sort)
}

func (s *Service) SetQuestionActive(ctx context.Context, id int64, active bool) error {
	return s.store.SetQuestionActive(ctx, id, active)
}

func (s *Service) String() string { return fmt.Sprintf("quiz service(%s)", s.title) }
