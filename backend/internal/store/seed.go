package store

import (
	"context"
	"encoding/json"
)

type seedQ struct {
	Content string   `json:"content"`
	Options []string `json:"options"`
	Answer  int      `json:"answer"`
	Score   int      `json:"score"`
}

func (s *Store) seedQuestions(ctx context.Context) error {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM questions`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	seeds := []seedQ{
		{"计算机中「CPU」的中文全称是？", []string{"中央处理器", "图形处理器", "内存控制器", "总线控制器"}, 0, 10},
		{"二进制数 1010 对应的十进制是？", []string{"8", "9", "10", "12"}, 2, 10},
		{"HTTP 默认端口号是？", []string{"21", "22", "80", "443"}, 2, 10},
		{"以下哪个是面向对象的编程语言？", []string{"C", "Java", "汇编语言", "SQL"}, 1, 10},
		{"WebSocket 是一种什么协议？", []string{"文件传输协议", "全双工通信协议", "邮件协议", "路由协议"}, 1, 10},
		{"RAM 的中文含义是？", []string{"只读存储器", "随机存取存储器", "高速缓存", "固态硬盘"}, 1, 10},
		{"IPv4 地址由多少位二进制组成？", []string{"16", "32", "64", "128"}, 1, 10},
		{"以下哪个命令用于查看 Linux 当前目录？", []string{"pwd", "ls -a .", "cd", "rm"}, 0, 10},
		{"一个字节（Byte）等于多少位（bit）？", []string{"4", "8", "16", "32"}, 1, 10},
		{"Git 中用于创建分支的命令是？", []string{"git branch", "git commit", "git clone", "git push"}, 0, 10},
	}
	for i, q := range seeds {
		raw, _ := json.Marshal(q.Options)
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO questions (content, options, answer, score, sort) VALUES ($1,$2,$3,$4,$5)`,
			q.Content, raw, q.Answer, q.Score, i); err != nil {
			return err
		}
	}
	return nil
}
