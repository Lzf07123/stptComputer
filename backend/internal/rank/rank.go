package rank

import (
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"
)

const (
	scoreKey     = "rank:score"
	nickKey      = "rank:nick"
	scale        = int64(1e13)
	maxElapsedMS = int64(6 * 3600 * 1000) // 6 小时封顶
)

type Rank struct{ rdb *redis.Client }

func New(addr string) *Rank {
	return &Rank{rdb: redis.NewClient(&redis.Options{Addr: addr})}
}

func (r *Rank) Ping(ctx context.Context) error { return r.rdb.Ping(ctx).Err() }

// EncodeScore 把「答对题数 + 用时」编码进单个 float 分数：
// 答对题数决定高位，用时短者同分排前。
func EncodeScore(correct int, elapsedMS int64) float64 {
	if elapsedMS < 0 {
		elapsedMS = 0
	}
	if elapsedMS > maxElapsedMS {
		elapsedMS = maxElapsedMS
	}
	return float64(correct)*float64(scale) + float64(maxElapsedMS-elapsedMS)
}

func DecodeScore(score float64) (correct int, elapsedMS int64) {
	s := int64(score)
	return int(s / scale), maxElapsedMS - (s % scale)
}

func (r *Rank) Update(ctx context.Context, userID int64, nickname string, correct int, elapsedMS int64) error {
	uid := strconv.FormatInt(userID, 10)
	pipe := r.rdb.TxPipeline()
	pipe.HSet(ctx, nickKey, uid, nickname)
	pipe.ZAdd(ctx, scoreKey, redis.Z{Score: EncodeScore(correct, elapsedMS), Member: uid})
	_, err := pipe.Exec(ctx)
	return err
}

type Entry struct {
	UserID    string  `json:"user_id"`
	Nickname  string  `json:"nickname"`
	Correct   int     `json:"correct"`
	ElapsedMS int64   `json:"elapsed_ms"`
	Score     float64 `json:"score"`
}

func (r *Rank) Top(ctx context.Context, n int64) ([]Entry, error) {
	if n <= 0 {
		return []Entry{}, nil
	}
	zs, err := r.rdb.ZRevRangeWithScores(ctx, scoreKey, 0, n-1).Result()
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(zs))
	for i, z := range zs {
		ids[i] = z.Member.(string)
	}
	nicks, _ := r.rdb.HMGet(ctx, nickKey, ids...).Result()
	out := make([]Entry, 0, len(zs))
	for i, z := range zs {
		nick := "匿名"
		if i < len(nicks) {
			if s, ok := nicks[i].(string); ok && s != "" {
				nick = s
			}
		}
		c, e := DecodeScore(z.Score)
		out = append(out, Entry{UserID: ids[i], Nickname: nick, Correct: c, ElapsedMS: e, Score: z.Score})
	}
	return out, nil
}

// RankOf 返回 0 起排名；未上榜返回 -1。
func (r *Rank) RankOf(ctx context.Context, userID int64) (int64, error) {
	uid := strconv.FormatInt(userID, 10)
	rnk, err := r.rdb.ZRevRank(ctx, scoreKey, uid).Result()
	if err == redis.Nil {
		return -1, nil
	}
	if err != nil {
		return -1, err
	}
	return rnk, nil
}

// Remove 从排行榜移除（放弃身份时调用）。
func (r *Rank) Remove(ctx context.Context, userID int64) error {
	return r.rdb.ZRem(ctx, scoreKey, strconv.FormatInt(userID, 10)).Err()
}

func (r *Rank) Count(ctx context.Context) (int64, error) {
	return r.rdb.ZCard(ctx, scoreKey).Result()
}
