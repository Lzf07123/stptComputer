import { useEffect, useRef, useState } from 'react'
import { Navigate } from 'react-router-dom'
import { api, clearToken, flushPending, getDeviceFP, getToken, loadPending, pushPending, setToken } from '../api/client'
import { QuizSocket, wsURL } from '../ws/socket'

interface Question {
  id: number
  content: string
  options: string[]
  score: number
}

interface AnswerRes {
  correct: boolean
  correct_answer: number
  score: number
  duplicate: boolean
}

type Phase = 'name' | 'loading' | 'quiz' | 'done'

export default function Quiz() {
  const [phase, setPhase] = useState<Phase>('name')
  const [nickname, setNickname] = useState('')
  const [questions, setQuestions] = useState<Question[]>([])
  const [idx, setIdx] = useState(0)
  const [selected, setSelected] = useState<number | null>(null)
  const [feedback, setFeedback] = useState<AnswerRes | null>(null)
  const [seconds, setSeconds] = useState(20)
  const [qSeconds, setQSeconds] = useState(20)
  const [error, setError] = useState('')
  const [flash, setFlash] = useState('')
  const locked = useRef(false)
  const sockRef = useRef<QuizSocket | null>(null)

  // 建立带 token 的实时连接（用于在线人次统计/特效/公告）
  const connectSocket = () => {
    const t = getToken()
    if (!t) return
    sockRef.current?.close()
    sockRef.current = new QuizSocket(`${wsURL()}?token=${encodeURIComponent(t)}`, () => undefined)
    sockRef.current.connect()
  }

  useEffect(() => () => sockRef.current?.close(), [])

  // 已有 token → 自动恢复（刷新页面/断网恢复）
  useEffect(() => {
    if (phase !== 'name') return
    if (!getToken()) return
    void resume()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function resume() {
    setPhase('loading')
    try {
      connectSocket()
      const flushed = await flushPending()
      if (flushed > 0) setFlash(`已补交 ${flushed} 条缓存的答案`)
      const [me, d] = await Promise.all([
        api<{ answered: number }>('/api/me'),
        api<{ questions: Question[]; question_seconds: number }>('/api/questions'),
      ])
      const answered = Math.min(me.answered, d.questions.length)
      setQuestions(d.questions)
      setQSeconds(d.question_seconds)
      setSeconds(d.question_seconds)
      if (answered >= d.questions.length) {
        setPhase('done')
      } else {
        setIdx(answered)
        setPhase('quiz')
      }
    } catch (e) {
      clearToken()
      setError((e as Error).message)
      setPhase('name')
    }
  }

  async function start() {
    setError('')
    setPhase('loading')
    try {
      const s = await api<{ token: string }>('/api/session', {
        method: 'POST',
        body: JSON.stringify({ nickname, device_fp: getDeviceFP() }),
      })
      setToken(s.token)
      connectSocket()
      const flushed = await flushPending()
      if (flushed > 0) setFlash(`已补交 ${flushed} 条缓存的答案`)
      const d = await api<{ questions: Question[]; question_seconds: number }>('/api/questions')
      if (d.questions.length === 0) {
        setError('题库为空，请联系现场工作人员')
        setPhase('name')
        return
      }
      setQuestions(d.questions)
      setQSeconds(d.question_seconds)
      setSeconds(d.question_seconds)
      setIdx(0)
      setPhase('quiz')
    } catch (e) {
      setError((e as Error).message)
      setPhase('name')
    }
  }

  async function answer(i: number | null, requestId: string) {
    const q = questions[idx]
    try {
      const res = await api<AnswerRes>('/api/answer', {
        method: 'POST',
        body: JSON.stringify({ request_id: requestId, question_id: q.id, selected: i }),
      })
      setFeedback(res)
      setSelected(i)
      setFlash('')
    } catch {
      // 提交失败：本地缓冲，不丢分
      pushPending({ request_id: requestId, question_id: q.id, selected: i })
      setFeedback({ correct: false, correct_answer: -1, score: 0, duplicate: false })
      setSelected(i)
      setFlash('网络异常，答案已缓存，恢复后自动补交')
    }
  }

  function choose(i: number) {
    if (locked.current || phase !== 'quiz' || feedback) return
    locked.current = true
    void answer(i, crypto.randomUUID())
  }

  function advance() {
    setFeedback(null)
    setSelected(null)
    locked.current = false
    if (idx + 1 >= questions.length) {
      setPhase('done')
    } else {
      setIdx(idx + 1)
      setSeconds(qSeconds)
    }
  }

  // 倒计时（出现作答反馈后停止，避免答完还要干等）
  useEffect(() => {
    if (phase !== 'quiz' || feedback) return
    const t = window.setInterval(() => {
      setSeconds((s) => (s <= 1 ? 0 : s - 1))
    }, 1000)
    return () => clearInterval(t)
  }, [phase, idx, feedback])

  // 作答后或超时后自动进入下一题
  useEffect(() => {
    if (phase !== 'quiz') return
    if (feedback) {
      const t = window.setTimeout(advance, 1200)
      return () => clearTimeout(t)
    }
    if (seconds <= 0) {
      const t = window.setTimeout(advance, 600)
      return () => clearTimeout(t)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [phase, feedback, seconds])

  if (phase === 'done') return <Navigate to="/result" replace />

  const q = phase === 'quiz' ? questions[idx] : null

  if (phase === 'quiz' && q) {
    const total = questions.length
    return (
      <section className="quiz-wrap">
        <div className="quiz-head">
          <span className="muted">
            第 {idx + 1} / {total} 题
          </span>
          <span className={`timer${seconds <= 5 ? ' danger' : ''}`}>{seconds}s</span>
        </div>
        <div className="progress">
          <div className="progress-bar" style={{ width: `${((idx + (feedback ? 1 : 0)) / total) * 100}%` }} />
        </div>
        <h2 className="question">{q.content}</h2>
        <div className="options">
          {q.options.map((opt, i) => {
            let cls = 'option'
            if (feedback) {
              if (i === feedback.correct_answer) cls += ' correct'
              else if (i === selected) cls += ' wrong'
            }
            return (
              <button key={i} className={cls} disabled={!!feedback} onClick={() => choose(i)}>
                <span className="opt-key">{String.fromCharCode(65 + i)}</span>
                {opt}
              </button>
            )
          })}
        </div>
        {feedback && (
          <p className={`feedback${feedback.correct ? ' ok' : ' bad'}`}>
            {feedback.correct ? `✔ 回答正确 +${feedback.score} 分` : '✘ 回答错误'}
          </p>
        )}
        {flash && <p className="flash">{flash}</p>}
      </section>
    )
  }

  return (
    <section className="card center">
      <h2>开始答题</h2>
      <p className="muted">输入昵称领取你的答题身份（每人限领一次）</p>
      <form
        className="name-form"
        onSubmit={(e) => {
          e.preventDefault()
          void start()
        }}
      >
        <input
          value={nickname}
          onChange={(e) => setNickname(e.target.value)}
          placeholder="输入昵称（1-16 字）"
          maxLength={16}
          autoFocus
        />
        <button className="btn btn-primary" type="submit" disabled={phase === 'loading'}>
          {phase === 'loading' ? '加载中…' : '进入答题'}
        </button>
      </form>
      {error && <p className="error">{error}</p>}
      {loadPending().length > 0 && getToken() === null && (
        <p className="muted">检测到 {loadPending().length} 条未提交答案，答题后将自动补交</p>
      )}
    </section>
  )
}
