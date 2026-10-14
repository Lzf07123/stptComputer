package publish

import (
	"context"

	"github.com/redis/go-redis/v9"
)

const EventsChannel = "quiz:events"

type Pub struct{ rdb *redis.Client }

func New(addr string) *Pub {
	return &Pub{rdb: redis.NewClient(&redis.Options{Addr: addr})}
}

// Publish 向所有订阅者广播一条 JSON 消息。
func (p *Pub) Publish(ctx context.Context, msg string) error {
	return p.rdb.Publish(ctx, EventsChannel, msg).Err()
}

// Subscribe 返回一个已订阅的 payload 通道，ctx 取消时自动退出。
func (p *Pub) Subscribe(ctx context.Context) <-chan string {
	ch := make(chan string, 128)
	ps := p.rdb.Subscribe(ctx, EventsChannel)
	go func() {
		defer close(ch)
		for m := range ps.Channel() {
			select {
			case ch <- m.Payload:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}
