package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"time"

	"stpt/quiz/internal/rank"
)

type envelope struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type Hub struct {
	rank       *rank.Rank
	events     <-chan string
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	clients    map[*Client]bool
	lastTop    []byte
	statsReq   chan chan [2]int
}

func NewHub(rk *rank.Rank, events <-chan string) *Hub {
	return &Hub{
		rank:       rk,
		events:     events,
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
		clients:    make(map[*Client]bool),
		statsReq:   make(chan chan [2]int),
	}
}

func (h *Hub) Run(ctx context.Context) {
	go h.tickRank(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case c := <-h.register:
			h.clients[c] = true
			if len(h.lastTop) > 0 {
				select {
				case c.send <- h.lastTop:
				default:
				}
			}
		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
		case msg := <-h.broadcast:
			for c := range h.clients {
				select {
				case c.send <- msg:
				default: // 慢客户端丢弃，避免阻塞 Hub
				}
			}
		case resp := <-h.statsReq:
			authed := 0
			for c := range h.clients {
				if c.authed {
					authed++
				}
			}
			resp <- [2]int{len(h.clients), authed}
		case ev := <-h.events:
			// 后台事件（公告/特效/答题状态），原样广播给所有连接
			if ev != "" {
				h.broadcast <- []byte(ev)
			}
		}
	}
}

// ClientCounts 返回 (总连接数, 已认证答题用户连接数)。
func (h *Hub) ClientCounts() (total, authed int) {
	ch := make(chan [2]int, 1)
	h.statsReq <- ch
	v := <-ch
	return v[0], v[1]
}

// tickRank 每 500ms 拉取 Top 20，仅在变化时广播（节流）。
func (h *Hub) tickRank(ctx context.Context) {
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			entries, err := h.rank.Top(ctx, 20)
			if err != nil {
				log.Printf("ws: top rank: %v", err)
				continue
			}
			b, err := json.Marshal(envelope{Type: "rank_update", Data: entries})
			if err != nil {
				continue
			}
			if !bytes.Equal(b, h.lastTop) {
				h.lastTop = b
				h.broadcast <- b
			}
		}
	}
}
