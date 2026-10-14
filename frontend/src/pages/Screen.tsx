import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, getToken } from '../api/client'
import { QuizSocket, wsURL } from '../ws/socket'

interface Entry {
  user_id: string
  nickname: string
  correct: number
  elapsed_ms: number
}

interface Stats {
  participants: number
  attempts: number
  online: number
}

interface MyRank {
  nickname: string
  rank: number
  total: number
}

export default function Screen() {
  const [top, setTop] = useState<Entry[]>([])
  const [stats, setStats] = useState<Stats>({ participants: 0, attempts: 0, online: 0 })
  const [connected, setConnected] = useState(false)
  const [effect, setEffect] = useState('')
  const [announce, setAnnounce] = useState('')
  const [state, setState] = useState('stop')
  const [myRank, setMyRank] = useState<MyRank | null>(null)

  useEffect(() => {
    const syncStats = () => {
      void api<Stats>('/api/stats')
        .then(setStats)
        .catch(() => undefined)
      // 本机若有答题身份，头部展示自己的全量排名
      if (getToken()) {
        void api<MyRank>('/api/me')
          .then(setMyRank)
          .catch(() => undefined)
      }
    }
    void syncStats()
    const st = window.setInterval(syncStats, 5000)

    const sock = new QuizSocket(
      wsURL(),
      (msg) => {
        if (msg.type === 'rank_update') setTop((msg.data || []) as Entry[])
        if (msg.type === 'effect') {
          setEffect((msg.data && msg.data.kind) || 'confetti')
          window.setTimeout(() => setEffect(''), 3000)
        }
        if (msg.type === 'announcement') {
          setAnnounce((msg.data && msg.data.content) || '')
          window.setTimeout(() => setAnnounce(''), 5000)
        }
        if (msg.type === 'quiz_state') setState((msg.data && msg.data.state) || 'stop')
      },
      (ok) => {
        setConnected(ok)
        if (ok) void syncStats()
      },
    )
    sock.connect()
    // 兜底轮询，防断流
    const t = window.setInterval(() => {
      void api<{ entries: Entry[] }>('/api/rank?top=99')
        .then((d) => setTop(d.entries))
        .catch(() => undefined)
    }, 10000)
    return () => {
      clearInterval(st)
      clearInterval(t)
      sock.close()
    }
  }, [])

  return (
    <section className={`screen${effect ? ' fx-' + effect : ''}`}>
      <header className="screen-head">
        <div className="screen-title">
          <Link className="screen-home" to="/">← 返回首页</Link>
          <h1>计算机协会 · 招新答题</h1>
        </div>
        <div className="screen-status">
          <span className={`dot${connected ? ' on' : ''}`} />
          {connected ? '实时连接' : '连接中…'}
          <span className={`badge${state === 'start' ? ' live' : ''}`}>
            {state === 'start' ? '答题进行中' : '等待开始'}
          </span>
        </div>
      </header>

      <div className="screen-stats">
        {myRank && (
          <span className="screen-me">
            我的排名 <b>{myRank.rank > 0 ? myRank.rank : '-'}</b> / 共 {myRank.total} 人
            <i className="screen-me-nick">{myRank.nickname}</i>
          </span>
        )}
        <span>参与答题 <b>{stats.participants}</b> 人</span>
        <span>累计答题 <b>{stats.attempts}</b> 次</span>
        <span>在线人次 <b>{stats.online}</b></span>
      </div>

      {announce && <div className="announce-banner"><span>{announce}</span></div>}

      <ol className="screen-rank">
        {top.map((e, i) => (
          <li key={e.user_id} className={`srow${i < 3 ? ` top${i + 1}` : ''}`}>
            <span className="srank">{i + 1}</span>
            <span className="sname" title={e.nickname}>{e.nickname}</span>
            <span className="sdetail">
              {e.correct} 题 · {(e.elapsed_ms / 1000).toFixed(0)}s
            </span>
          </li>
        ))}
        {top.length === 0 && <li className="muted center">等待第一批勇士上榜…</li>}
      </ol>

      {effect && <div className="effect-overlay">🎉</div>}
    </section>
  )
}
