import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, clearToken } from '../api/client'

interface Me {
  user_id: number
  nickname: string
  correct: number
  answered: number
  elapsed_ms: number
  score: number
  rank: number
  total: number
}

interface Entry {
  user_id: string
  nickname: string
  correct: number
  elapsed_ms: number
}

export default function Result() {
  const nav = useNavigate()
  const [me, setMe] = useState<Me | null>(null)
  const [top, setTop] = useState<Entry[]>([])

  useEffect(() => {
    void (async () => {
      try {
        const [m, r] = await Promise.all([api<Me>('/api/me'), api<{ entries: Entry[] }>('/api/rank?top=10')])
        setMe(m)
        setTop(r.entries)
      } catch {
        setMe(null)
      }
    })()
  }, [])

  function fmt(ms: number) {
    const s = Math.round(ms / 1000)
    return `${Math.floor(s / 60)}分${s % 60}秒`
  }

  if (!me) {
    return (
      <section className="card center">
        <h2>暂无答题记录</h2>
        <p className="muted">请先完成答题</p>
        <button className="btn btn-primary" onClick={() => nav('/quiz')}>
          去答题
        </button>
      </section>
    )
  }

  return (
    <section className="result-wrap">
      <div className="card center score-card">
        <h2>{me.nickname}</h2>
        <p className="big-score">{me.score}</p>
        <p className="muted">
          答对 {me.correct} / {me.answered} 题 · 用时 {fmt(me.elapsed_ms)}
        </p>
        <p className="rank-line">
          当前排名 <b>{me.rank > 0 ? `第 ${me.rank} 名` : '未上榜'}</b> / 共 {me.total} 人参与
        </p>
        <button
          className="btn"
          onClick={() => {
            clearToken()
            nav('/quiz')
          }}
        >
          再来一次
        </button>
      </div>

      <div className="card">
        <h3>实时排行榜 TOP 10</h3>
        <ol className="rank-list">
          {top.map((e, i) => (
            <li key={e.user_id} className={e.user_id === String(me.user_id) ? 'self' : ''}>
              <span className={`rank-no no${i + 1}`}>{i + 1}</span>
              <span className="rank-name">{e.nickname}</span>
              <span className="rank-detail">
                {e.correct} 题 · {fmt(e.elapsed_ms)}
              </span>
            </li>
          ))}
          {top.length === 0 && <li className="muted">暂无数据</li>}
        </ol>
      </div>
    </section>
  )
}
