import { useEffect, useState } from 'react'
import { api } from '../api/client'
import { QuizSocket, wsURL } from '../ws/socket'

interface Entry {
  user_id: string
  nickname: string
  correct: number
  elapsed_ms: number
}

export default function Screen() {
  const [top, setTop] = useState<Entry[]>([])
  const [connected, setConnected] = useState(false)
  const [effect, setEffect] = useState('')
  const [announce, setAnnounce] = useState('')
  const [state, setState] = useState('stop')

  useEffect(() => {
    const syncState = () => {
      void api<{ quiz_state: string }>('/api/config')
        .then((c) => setState(c.quiz_state))
        .catch(() => undefined)
    }
    void syncState() // 初次进入同步一次（事件只广播增量）
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
        if (ok) void syncState() // 重连成功后重新同步
      },
    )
    sock.connect()
    // 兜底轮询，防断流
    const t = window.setInterval(() => {
      void api<{ entries: Entry[] }>('/api/rank?top=50')
        .then((d) => setTop(d.entries))
        .catch(() => undefined)
    }, 10000)
    return () => {
      clearInterval(t)
      sock.close()
    }
  }, [])

  return (
    <section className={`screen${effect ? ' fx-' + effect : ''}`}>
      <header className="screen-head">
        <h1>计算机协会 · 招新答题</h1>
        <div className="screen-status">
          <span className={`dot${connected ? ' on' : ''}`} />
          {connected ? '实时连接' : '连接中…'}
          <span className={`badge${state === 'start' ? ' live' : ''}`}>
            {state === 'start' ? '答题进行中' : '等待开始'}
          </span>
        </div>
      </header>

      {announce && <div className="announce-banner">{announce}</div>}

      <ol className="screen-rank">
        {top.map((e, i) => (
          <li key={e.user_id} className={`srow${i < 3 ? ` top${i + 1}` : ''}`}>
            <span className="srank">{i + 1}</span>
            <span className="sname">{e.nickname}</span>
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
