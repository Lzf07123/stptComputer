package store

import (
	"context"
	"encoding/json"
)

type seedQ struct {
	Content    string   `json:"content"`
	Options    []string `json:"options"`
	Answer     int      `json:"answer"`
	Score      int      `json:"score"`
	Difficulty string   `json:"difficulty"`
}

// seedQuestions 题库为空时整体播种；已存在同名题目时跳过（幂等，可增量扩充）。
func (s *Store) seedQuestions(ctx context.Context) error {
	seeds := []seedQ{
		// ---- 基础 10 题 ----
		{"计算机中「CPU」的中文全称是？", []string{"中央处理器", "图形处理器", "内存控制器", "总线控制器"}, 0, 10, "容易"},
		{"二进制数 1010 对应的十进制是？", []string{"8", "9", "10", "12"}, 2, 10, "容易"},
		{"HTTP 默认端口号是？", []string{"21", "22", "80", "443"}, 2, 10, "容易"},
		{"以下哪个是面向对象的编程语言？", []string{"C", "Java", "汇编语言", "SQL"}, 1, 10, "简单"},
		{"WebSocket 是一种什么协议？", []string{"文件传输协议", "全双工通信协议", "邮件协议", "路由协议"}, 1, 10, "简单"},
		{"RAM 的中文含义是？", []string{"只读存储器", "随机存取存储器", "高速缓存", "固态硬盘"}, 1, 10, "容易"},
		{"IPv4 地址由多少位二进制组成？", []string{"16", "32", "64", "128"}, 1, 10, "容易"},
		{"以下哪个命令用于查看 Linux 当前目录？", []string{"pwd", "ls -a .", "cd", "rm"}, 0, 10, "容易"},
		{"一个字节（Byte）等于多少位（bit）？", []string{"4", "8", "16", "32"}, 1, 10, "容易"},
		{"Git 中用于创建分支的命令是？", []string{"git branch", "git commit", "git clone", "git push"}, 0, 10, "简单"},
		// ---- 扩充 30 题（正确项固定首位，前台展示时按用户乱序） ----
		{"DNS 的主要作用是什么？", []string{"将域名解析为 IP 地址", "加密网页内容", "存储网站文件", "压缩图片"}, 0, 10, "容易"},
		{"HTTPS 相比 HTTP 的主要优势是？", []string{"数据加密传输更安全", "打开网页速度更快", "只能用于视频网站", "不需要域名"}, 0, 10, "简单"},
		{"冯·诺依曼体系结构的核心思想是？", []string{"存储程序与程序控制", "使用晶体管代替电子管", "完全基于云计算", "依靠神经网络计算"}, 0, 10, "困难"},
		{"数据结构中「栈」的特点是？", []string{"后进先出", "先进先出", "随机存取", "只能存储数字"}, 0, 10, "容易"},
		{"数据结构中「队列」的特点是？", []string{"先进先出", "后进先出", "随机存取", "只能存储字符"}, 0, 10, "简单"},
		{"冒泡排序最坏情况的时间复杂度是？", []string{"O(n²)", "O(n log n)", "O(n)", "O(1)"}, 0, 10, "困难"},
		{"二分查找的前提条件是？", []string{"数据必须有序", "必须使用链表存储", "数据量必须超过一万条", "数据必须乱序"}, 0, 10, "容易"},
		{"SQL 中用于查询数据的关键字是？", []string{"SELECT", "INSERT", "UPDATE", "DELETE"}, 0, 10, "简单"},
		{"数据库表主键（Primary Key）的作用是？", []string{"唯一标识表中的每条记录", "提高硬盘读写速度", "存储图片等大文件", "对数据进行加密"}, 0, 10, "困难"},
		{"TCP 与 UDP 的主要区别是？", []string{"TCP 面向连接且可靠传输", "UDP 比 TCP 速度更慢", "TCP 不保证数据顺序", "UDP 面向连接"}, 0, 10, "容易"},
		{"路由器工作在 OSI 模型的哪一层？", []string{"网络层", "物理层", "应用层", "传输层"}, 0, 10, "简单"},
		{"CSS 中设置文字颜色的属性是？", []string{"color", "font-size", "border", "margin"}, 0, 10, "困难"},
		{"JavaScript 中用于声明变量的关键字是？", []string{"let", "printf", "int", "define"}, 0, 10, "容易"},
		{"Python 中用于输出内容的函数是？", []string{"print()", "echo()", "printf()", "cout"}, 0, 10, "简单"},
		{"操作系统中「死锁」指的是？", []string{"两个进程互相等待对方占用的资源", "内存空间不足", "CPU 温度过高", "网络连接断开"}, 0, 10, "困难"},
		{"虚拟内存的主要作用是？", []string{"让程序可用地址空间超出物理内存", "提高硬盘的转速", "加快显卡的渲染速度", "减少 CPU 的核心数量"}, 0, 10, "容易"},
		{"HTTPS 默认使用的端口号是？", []string{"443", "80", "21", "25"}, 0, 10, "简单"},
		{"防火墙的主要功能是？", []string{"按规则过滤网络流量", "加速网络传输", "存储网页内容", "压缩数据文件"}, 0, 10, "困难"},
		{"软件测试阶段的主要目的是？", []string{"发现并定位软件缺陷", "编写程序代码", "部署上线服务器", "设计数据库结构"}, 0, 10, "容易"},
		{"递归函数的特点是？", []string{"函数直接或间接调用自身", "只能调用标准库函数", "不能有任何参数", "一定会无限循环"}, 0, 10, "简单"},
		{"哈希表平均查找的时间复杂度是？", []string{"O(1)", "O(n)", "O(log n)", "O(n²)"}, 0, 10, "困难"},
		{"以下排序算法中平均性能较好、常用于通用排序的是？", []string{"快速排序", "冒泡排序", "选择排序", "插入排序"}, 0, 10, "容易"},
		{"一个 ASCII 字符在计算机中通常占用多少字节存储？", []string{"1 字节", "2 字节", "4 字节", "8 字节"}, 0, 10, "简单"},
		{"编译型语言的特点是？", []string{"编译为机器码后运行", "逐行解释执行", "只能用于网页开发", "完全不需要编译器"}, 0, 10, "困难"},
		{"数据库事务 ACID 特性中的「A」代表？", []string{"原子性", "可用性", "加速性", "可视化"}, 0, 10, "容易"},
		{"物联网英文缩写 IoT 的中文含义是？", []string{"物联网", "星际传输", "局域网协议", "办公自动化"}, 0, 10, "简单"},
		{"Unicode 编码的主要作用是？", []string{"为全球各种字符统一编码", "加快网络传输速度", "压缩图片文件", "管理计算机内存"}, 0, 10, "困难"},
		{"客户端-服务器（C/S）架构的特点是？", []string{"客户端发起请求、服务器响应处理", "两台电脑互为客户端", "通信完全不需要网络", "服务器就是显示器"}, 0, 10, "容易"},
		{"「开源软件」指的是？", []string{"源代码公开且可查看修改", "一定完全免费", "禁止用于商业用途", "只能由个人开发"}, 0, 10, "简单"},
		{"局域网 LAN 的英文全称是？", []string{"Local Area Network", "Long Area Network", "Large Access Network", "Local Access Node"}, 0, 10, "困难"},
	}

	for i, q := range seeds {
		var exists bool
		if err := s.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM questions WHERE content = $1)`, q.Content).Scan(&exists); err != nil {
			return err
		}
		if exists {
			// 种子题目内容为准，同步难度（用户自建题不受影响）
			if _, err := s.pool.Exec(ctx,
				`UPDATE questions SET difficulty = $2 WHERE content = $1`,
				q.Content, q.Difficulty); err != nil {
				return err
			}
			continue
		}
		raw, _ := json.Marshal(q.Options)
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO questions (content, options, answer, score, sort, difficulty) VALUES ($1,$2,$3,$4,$5,$6)`,
			q.Content, raw, q.Answer, q.Score, i, q.Difficulty); err != nil {
			return err
		}
	}
	return nil
}
