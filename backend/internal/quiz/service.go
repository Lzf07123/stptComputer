package quiz

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	mathrand "math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"stpt/quiz/internal/publish"
	"stpt/quiz/internal/rank"
	"stpt/quiz/internal/store"
)

var (
	ErrQuizNotStarted = errors.New("答题未开始")
	ErrNoRound        = errors.New("请先开始一轮答题")
	ErrIdentityBound  = errors.New("该设备已绑定答题身份")
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

func (s *Service) StartSession(ctx context.Context, nickname, deviceFP string) (*store.User, bool, error) {
	nick := strings.TrimSpace(nickname)
	r := []rune(nick)
	if len(r) == 0 || len(r) > 16 {
		return nil, false, errors.New("昵称长度需为 1-16 个字符")
	}
	for _, c := range nick {
		if c < 0x20 || c == 0x7f {
			return nil, false, errors.New("昵称含非法字符")
		}
	}
	if containsBadWord(nick) {
		return nil, false, errors.New("昵称含敏感词，请更换")
	}
	// 同一设备只允许一个活跃答题身份：
	// 同昵称 = 本机恢复原身份；不同昵称 = 拒绝，不侵占原身份，需先放弃
	if deviceFP != "" {
		if ex, err := s.store.GetUserByDeviceFP(ctx, deviceFP); err == nil && ex != nil {
			if ex.Nickname == nick {
				return ex, true, nil
			}
			return nil, false, fmt.Errorf("%w「%s」", ErrIdentityBound, ex.Nickname)
		}
	}
	// 昵称全局唯一：已放弃的身份永久封存，任何人都不得复用（含本人）
	if ex, err := s.store.GetUserByNickname(ctx, nick); err == nil && ex != nil {
		if ex.Abandoned {
			return nil, false, errors.New("该昵称已随身份放弃，不可再次使用")
		}
		return nil, false, errors.New("该昵称已被使用，请更换")
	}
	token := randomHex(32)
	u, err := s.store.CreateUser(ctx, nick, token, deviceFP)
	return u, false, err
}

func (s *Service) ValidateToken(ctx context.Context, token string) (*store.User, error) {
	if token == "" {
		return nil, ErrBadRequest
	}
	u, err := s.store.GetUserByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if u.Abandoned {
		return nil, errors.New("身份已放弃")
	}
	return u, nil
}

// Abandon 放弃身份：必须证明该身份属于本机（设备指纹一致），否则拒绝。
// 排行榜记录保留（历史足迹），仅身份失效：token 作废、设备解绑、昵称封存。
func (s *Service) Abandon(ctx context.Context, userID int64, deviceFP string) error {
	u, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	if u.DeviceFP == "" || deviceFP == "" || u.DeviceFP != deviceFP {
		return errors.New("不可放弃不属于本机的身份")
	}
	return s.store.AbandonUser(ctx, userID)
}

// BoundIdentity 返回该设备当前绑定的活跃身份；未绑定返回 store.ErrNotFound。
func (s *Service) BoundIdentity(ctx context.Context, deviceFP string) (*store.User, error) {
	return s.store.GetUserByDeviceFP(ctx, deviceFP)
}

// AbandonByDeviceFP 按设备指纹放弃该设备当前绑定的身份（无身份则静默成功）。
func (s *Service) AbandonByDeviceFP(ctx context.Context, deviceFP string) error {
	if deviceFP == "" {
		return ErrBadRequest
	}
	u, err := s.store.GetUserByDeviceFP(ctx, deviceFP)
	if err != nil || u == nil {
		return nil
	}
	// 本机路径：设备指纹即归属证明；排行榜记录保留
	return s.store.AbandonUser(ctx, u.ID)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = cryptorand.Read(b)
	return hex.EncodeToString(b)
}

func containsBadWord(s string) bool {
	lower := strings.ToLower(strings.Join(strings.Fields(s), ""))
	bad := []string{
		// 中文辱骂
		"傻逼", "傻比", "傻叉", "傻屄", "弱智", "智障", "脑残", "白痴", "蠢货", "蠢猪",
		"废物", "垃圾", "贱人", "贱货", "婊子", "骚货", "畜生", "禽兽", "杂种", "狗东西",
		"狗日的", "狗娘养", "龟儿子", "操你", "草你", "肏你", "艹你", "日你", "你妈", "你麻痹",
		"尼玛", "他妈", "他妈逼", "你妹", "滚犊子", "去死", "死全家", "断子绝孙",
		"屁眼", "屁精", "龟孙", "王八", "王八蛋", "狗屁", "放屁", "傻蛋", "憨批", "憨憨",
		"屌丝", "屌毛", "装逼", "装比", "撕逼", "撕比", "二逼", "逗比", "煞笔", "沙雕",
		"贱婢", "野种", "孽种", "狗屎", "吃屎", "吃翔", "我操", "我草", "我日", "丢你",
		"妈蛋", "妈逼", "祖宗十八代", "傻白甜", "二百五", "没妈", "没爹", "孤儿",
		// 英文及缩写（含去空格变体）
		"fuck", "fuk", "fck", "shit", "bitch", "bi tch", "asshole", "ass hole",
		"bastard", "dickhead", "cunt", "nigger", "nigga", "whore", "slut", "motherfucker",
		"nmsl", "cnm", "sb", "tmd", "wcnm", "qnmd", "ntmd", "nmd", "ggb", "mmd",
	}
	for _, b := range bad {
		if strings.Contains(lower, b) {
			return true
		}
	}
	return false
}

// ---------- questions ----------

// orderedOptions 按用户维度确定性地打乱选项，提交判分时用同一顺序还原。
func orderedOptions(q store.Question, userID int64, roundNo int) []string {
	opts := append([]string(nil), q.Options...)
	r := mathrand.New(mathrand.NewSource(userID*104729 + int64(q.ID)*31 + int64(roundNo)*17 + 7))
	r.Shuffle(len(opts), func(i, j int) { opts[i], opts[j] = opts[j], opts[i] })
	return opts
}

// drawQuestions 按所选难度筛选 + (用户, 轮次) 确定性随机抽 limit 题。
// 同轮刷新结果一致（保证断线恢复），不同轮次/不同用户题目不同。
func (s *Service) drawQuestions(ctx context.Context, userID int64, roundNo int, difficulty string, limit int) ([]store.Question, error) {
	qs, err := s.store.ListActiveQuestions(ctx, difficulty)
	if err != nil {
		return nil, err
	}
	r := mathrand.New(mathrand.NewSource(userID*7919 + int64(roundNo)*104729 + 13))
	r.Shuffle(len(qs), func(i, j int) { qs[i], qs[j] = qs[j], qs[i] })
	if limit > 0 && len(qs) > limit {
		qs = qs[:limit]
	}
	for i := range qs {
		qs[i].Options = orderedOptions(qs[i], userID, roundNo)
	}
	return qs, nil
}

// AbandonRound 放弃本轮答题：作废本轮成绩，身份与历史最佳轮保留。
func (s *Service) AbandonRound(ctx context.Context, userID int64) (int64, error) {
	roundNo, err := s.store.CurrentRoundNo(ctx, userID)
	if err != nil {
		return 0, err
	}
	if roundNo == 0 {
		return 0, ErrNoRound
	}
	deleted, err := s.store.AbandonRound(ctx, userID, roundNo)
	if err != nil {
		return 0, err
	}
	// 本轮可能是历史最佳，作废后按剩余轮次重算榜单
	go s.refreshRank(context.WithoutCancel(ctx), userID)
	return deleted, nil
}

// StartRound 开始新一轮：难度由用户答题前选择并随轮持久化。
func (s *Service) StartRound(ctx context.Context, userID int64, difficulty string, limit int) (int, []store.Question, error) {
	if !validDifficulty(difficulty) {
		return 0, nil, ErrBadRequest
	}
	roundNo, err := s.store.NextRound(ctx, userID, difficulty)
	if err != nil {
		return 0, nil, err
	}
	qs, err := s.drawQuestions(ctx, userID, roundNo, difficulty, limit)
	if err != nil {
		return 0, nil, err
	}
	return roundNo, qs, nil
}

// CurrentRound 返回当前进行中的一轮；无轮次或本轮已答完返回 ErrNoRound。
func (s *Service) CurrentRound(ctx context.Context, userID int64, limit int) (int, []store.Question, int, error) {
	roundNo, err := s.store.CurrentRoundNo(ctx, userID)
	if err != nil {
		return 0, nil, 0, err
	}
	if roundNo == 0 {
		return 0, nil, 0, ErrNoRound
	}
	difficulty, err := s.store.CurrentDifficulty(ctx, userID)
	if err != nil {
		return 0, nil, 0, err
	}
	qs, err := s.drawQuestions(ctx, userID, roundNo, difficulty, limit)
	if err != nil {
		return 0, nil, 0, err
	}
	st, err := s.store.RoundStats(ctx, userID, roundNo)
	if err != nil {
		return 0, nil, 0, err
	}
	if st.Count >= len(qs) {
		return 0, nil, 0, ErrNoRound
	}
	return roundNo, qs, st.Count, nil
}

// correctIndex 返回该用户视角下正确答案的下标。
func correctIndex(q store.Question, userID int64, roundNo int) int {
	order := orderedOptions(q, userID, roundNo)
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
	roundNo, err := s.store.CurrentRoundNo(ctx, userID)
	if err != nil {
		return nil, -1, false, err
	}
	if roundNo == 0 {
		return nil, -1, false, ErrNoRound
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
		order := orderedOptions(*q, userID, roundNo)
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
		RoundNo:    roundNo,
	})
	if err != nil {
		return nil, -1, false, err
	}
	if !duplicate {
		// 异步刷新排行榜，不阻塞答题提交关键路径
		go s.refreshRank(context.WithoutCancel(ctx), userID)
	}
	return att, correctIndex(*q, userID, roundNo), duplicate, nil
}

func (s *Service) refreshRank(ctx context.Context, userID int64) {
	stats, err := s.store.BestRoundStats(ctx, userID)
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
	st, err := s.store.BestRoundStats(ctx, userID)
	if err != nil {
		return nil, err
	}
	elapsed := int64(0)
	if st.Count > 0 {
		elapsed = st.Last.Sub(st.First).Milliseconds()
	}
	rnk, total, err := s.RankOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &MyStats{
		UserID:    u.ID,
		Nickname:  u.Nickname,
		Correct:   st.Correct,
		Answered:  st.Count,
		ElapsedMS: elapsed,
		Score:     st.Correct * 10, // 最佳一轮成绩
		Rank:      rnk,             // 全量排名（含未答题者）
		Total:     total,
	}, nil
}

// 全量榜缓存：读写分离 + 后台定时刷新；读请求永不被 DB 查询阻塞，
// 高并发下每 1s 最多一次 DB 聚合，WS 推送/轮询/个人名次共用。
var (
	lbMu    sync.RWMutex
	lbCache = struct {
		entries []rank.Entry
		total   int64
		at      time.Time
	}{}
	lbRefreshMu sync.Mutex
	lbTTL       = 1 * time.Second
)

func (s *Service) refreshLeaderboard(ctx context.Context) {
	lbRefreshMu.Lock()
	defer lbRefreshMu.Unlock()
	// 单飞：获取刷新锁后复查新鲜度，避免过期瞬间大量请求各自重查库
	lbMu.RLock()
	fresh := lbCache.entries != nil && time.Since(lbCache.at) < lbTTL
	lbMu.RUnlock()
	if fresh {
		return
	}
	rows, total, err := s.store.Leaderboard(ctx)
	if err != nil {
		return
	}
	entries := make([]rank.Entry, 0, len(rows))
	for _, r := range rows {
		entries = append(entries, rank.Entry{
			UserID:    strconv.FormatInt(r.UserID, 10),
			Nickname:  r.Nickname,
			Correct:   r.Correct,
			ElapsedMS: r.ElapsedMS,
		})
	}
	lbMu.Lock()
	lbCache.entries, lbCache.total, lbCache.at = entries, total, time.Now()
	lbMu.Unlock()
}

// StartLeaderboardRefresh 启动后台刷新（main 中调用一次）。
func (s *Service) StartLeaderboardRefresh(ctx context.Context) {
	go func() {
		s.refreshLeaderboard(ctx) // 首次同步，避免启动即空
		t := time.NewTicker(lbTTL)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.refreshLeaderboard(ctx)
			}
		}
	}()
}

func (s *Service) fullLeaderboard(ctx context.Context) ([]rank.Entry, int64, error) {
	// 读请求永不阻塞：有数据即返回（即使刚过期，后台会刷新）；无数据才同步刷新一次
	lbMu.RLock()
	if lbCache.entries != nil {
		e, t := lbCache.entries, lbCache.total
		lbMu.RUnlock()
		return e, t, nil
	}
	lbMu.RUnlock()
	s.refreshLeaderboard(ctx)
	lbMu.RLock()
	defer lbMu.RUnlock()
	return lbCache.entries, lbCache.total, nil
}

// Top 供 WebSocket 推送使用：全量排行榜前 n 名（包含未答题者）。
func (s *Service) Top(ctx context.Context, n int64) ([]rank.Entry, error) {
	entries, _, err := s.Leaderboard(ctx, int(n))
	return entries, err
}

// Leaderboard 返回前 limit 名与总人数（limit<=0 返回全部）。
func (s *Service) Leaderboard(ctx context.Context, limit int) ([]rank.Entry, int64, error) {
	entries, total, err := s.fullLeaderboard(ctx)
	if err != nil {
		return nil, 0, err
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, total, nil
}

// RankOf 返回用户在全量排行榜中的名次（1 起）与总人数。
func (s *Service) RankOf(ctx context.Context, userID int64) (int64, int64, error) {
	entries, total, err := s.fullLeaderboard(ctx)
	if err != nil {
		return 0, 0, err
	}
	for i, e := range entries {
		if e.UserID == strconv.FormatInt(userID, 10) {
			return int64(i + 1), total, nil
		}
	}
	return 0, total, nil
}

// ---------- stats ----------

type Stats struct {
	Participants int64 `json:"participants"` // 参与答题人数
	Attempts     int64 `json:"attempts"`     // 累计答题次数
}

func (s *Service) Stats(ctx context.Context) (*Stats, error) {
	p, err := s.store.CountAnsweredUsers(ctx)
	if err != nil {
		return nil, err
	}
	a, err := s.store.CountAttempts(ctx)
	if err != nil {
		return nil, err
	}
	return &Stats{Participants: p, Attempts: a}, nil
}

// ---------- footer ----------

func (s *Service) Footer(ctx context.Context) (*store.Footer, error) {
	return s.store.GetFooterSettings(ctx)
}

func (s *Service) SaveFooter(ctx context.Context, f *store.Footer) error {
	f.Owner = strings.TrimSpace(f.Owner)
	f.License = strings.TrimSpace(f.License)
	f.ICP = strings.TrimSpace(f.ICP)
	f.Police = strings.TrimSpace(f.Police)
	if len([]rune(f.Owner)) > 100 || len([]rune(f.License)) > 100 ||
		len([]rune(f.ICP)) > 60 || len([]rune(f.Police)) > 60 ||
		len([]rune(f.ICPIcon)) > 200 || len([]rune(f.PoliceIcon)) > 200 {
		return ErrBadRequest
	}
	return s.store.SaveFooterSettings(ctx, f)
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

// Qrcodes 返回前台「加入我们」页展示的启用二维码。
func (s *Service) Qrcodes(ctx context.Context) ([]store.Qrcode, error) {
	return s.store.ListActiveQrcodes(ctx)
}

// AllQrcodes 返回全部二维码（后台管理）。
func (s *Service) AllQrcodes(ctx context.Context) ([]store.Qrcode, error) {
	return s.store.ListQrcodes(ctx)
}

// AddQrcode 新增二维码（默认启用，标题可为空）。
func (s *Service) AddQrcode(ctx context.Context, title, imageURL string) (*store.Qrcode, error) {
	title = strings.TrimSpace(title)
	if strings.TrimSpace(imageURL) == "" || len([]rune(title)) > 30 {
		return nil, ErrBadRequest
	}
	return s.store.CreateQrcode(ctx, title, imageURL)
}

// UpdateQrcode 局部更新标题/图片/启用状态；图片被替换时返回旧路径供调用方清理。
func (s *Service) UpdateQrcode(ctx context.Context, id int64, title, imageURL *string, active *bool) (string, error) {
	q, err := s.store.GetQrcode(ctx, id)
	if err != nil {
		return "", err
	}
	newTitle, newImage, newActive := q.Title, q.ImageURL, q.Active
	if title != nil {
		newTitle = strings.TrimSpace(*title)
		if len([]rune(newTitle)) > 30 {
			return "", ErrBadRequest
		}
	}
	if imageURL != nil {
		newImage = strings.TrimSpace(*imageURL)
		if newImage == "" {
			return "", ErrBadRequest
		}
	}
	if active != nil {
		newActive = *active
	}
	if err := s.store.UpdateQrcode(ctx, id, newTitle, newImage, newActive); err != nil {
		return "", err
	}
	if newImage != q.ImageURL {
		return q.ImageURL, nil
	}
	return "", nil
}

// DeleteQrcode 删除二维码并返回图片路径供清理。
func (s *Service) DeleteQrcode(ctx context.Context, id int64) (string, error) {
	return s.store.DeleteQrcode(ctx, id)
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

// ---------- overview ----------

type Overview struct {
	Users        int64  `json:"users"`
	Attempts     int64  `json:"attempts"`
	Participants int64  `json:"participants"`
	Online       int64  `json:"online"`
	Questions    int64  `json:"questions"`
	Backgrounds  int64  `json:"backgrounds"`
	DB           string `json:"db"`
	Redis        string `json:"redis"`
}

func (s *Service) Overview(ctx context.Context, online int64) (*Overview, error) {
	users, err := s.store.CountUsers(ctx)
	if err != nil {
		return nil, err
	}
	attempts, err := s.store.CountAttempts(ctx)
	if err != nil {
		return nil, err
	}
	participants, err := s.store.CountAnsweredUsers(ctx)
	if err != nil {
		return nil, err
	}
	questions, err := s.store.CountQuestions(ctx)
	if err != nil {
		return nil, err
	}
	bgs, err := s.store.CountBackgrounds(ctx)
	if err != nil {
		return nil, err
	}
	db, redisStatus := "ok", "ok"
	if err := s.store.Ping(ctx); err != nil {
		db = "error"
	}
	if err := s.rank.Ping(ctx); err != nil {
		redisStatus = "error"
	}
	return &Overview{
		Users: users, Attempts: attempts, Participants: participants,
		Online: online, Questions: questions, Backgrounds: bgs,
		DB: db, Redis: redisStatus,
	}, nil
}

// ---------- export scores ----------

type ScoreRow struct {
	Rank      int64     `json:"rank"`
	Nickname  string    `json:"nickname"`
	Correct   int       `json:"correct"`
	Answered  int       `json:"answered"`
	ElapsedMS int64     `json:"elapsed_ms"`
	Score     int       `json:"score"`
	Rounds    int       `json:"rounds"`
	Abandoned bool      `json:"abandoned"`
	CreatedAt time.Time `json:"created_at"`
}

// ExportScores 导出全部用户的最高成绩（按榜单名次，未上榜按注册顺序排后）。
func (s *Service) ExportScores(ctx context.Context) ([]ScoreRow, error) {
	users, err := s.store.ListAllUsers(ctx)
	if err != nil {
		return nil, err
	}
	rounds, err := s.store.RoundCounts(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := s.rank.Top(ctx, 100000)
	if err != nil {
		return nil, err
	}
	pos := map[string]int64{}
	for i, e := range entries {
		pos[e.UserID] = int64(i + 1)
	}
	ranked := []ScoreRow{}
	unranked := []ScoreRow{}
	for _, u := range users {
		st, err := s.store.BestRoundStats(ctx, u.ID)
		if err != nil {
			continue
		}
		elapsed := int64(0)
		if st.Count > 0 {
			elapsed = st.Last.Sub(st.First).Milliseconds()
		}
		row := ScoreRow{
			Nickname:  u.Nickname,
			Correct:   st.Correct,
			Answered:  st.Count,
			ElapsedMS: elapsed,
			Score:     st.Correct * 10,
			Rounds:    rounds[u.ID],
			Abandoned: u.Abandoned,
			CreatedAt: u.CreatedAt,
		}
		if p, ok := pos[fmt.Sprint(u.ID)]; ok {
			row.Rank = p
			ranked = append(ranked, row)
		} else {
			unranked = append(unranked, row)
		}
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].Rank < ranked[j].Rank })
	return append(ranked, unranked...), nil
}

// ---------- difficulty ----------

func validDifficulty(d string) bool {
	return d == "全部" || d == "容易" || d == "简单" || d == "困难"
}

func (s *Service) Difficulty(ctx context.Context) (string, error) {
	d, err := s.store.QuizDifficulty(ctx)
	if err != nil {
		return "全部", err
	}
	return d, nil
}

func (s *Service) SetDifficulty(ctx context.Context, d string) error {
	if !validDifficulty(d) {
		return ErrBadRequest
	}
	return s.store.SetQuizDifficulty(ctx, d)
}

// ClearQuestions 一键清空题库（答题记录与排行榜保留）。
func (s *Service) ClearQuestions(ctx context.Context) (int64, error) {
	return s.store.ClearQuestions(ctx)
}

// ClearBackgrounds 一键清空背景图（返回文件路径供清理磁盘）。
func (s *Service) ClearBackgrounds(ctx context.Context) ([]string, error) {
	return s.store.DeleteAllBackgrounds(ctx)
}

// ---------- admin: questions ----------

func (s *Service) AllQuestions(ctx context.Context) ([]store.Question, error) {
	return s.store.AllQuestions(ctx)
}

func (s *Service) CreateQuestion(ctx context.Context, content string, options []string, answer, score, sort int, difficulty string) (*store.Question, error) {
	content = strings.TrimSpace(content)
	if content == "" || len(options) < 2 || len(options) > 6 || answer < 0 || answer >= len(options) {
		return nil, ErrBadRequest
	}
	if !validDifficulty(difficulty) || difficulty == "全部" {
		return nil, ErrBadRequest
	}
	if score <= 0 {
		score = 10
	}
	return s.store.CreateQuestion(ctx, content, options, answer, score, sort, difficulty)
}

func (s *Service) SetQuestionActive(ctx context.Context, id int64, active bool) error {
	return s.store.SetQuestionActive(ctx, id, active)
}

func (s *Service) String() string { return fmt.Sprintf("quiz service(%s)", s.title) }
