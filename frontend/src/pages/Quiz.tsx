import { useEffect, useRef, useState } from 'react'
import { Navigate } from 'react-router-dom'
import { api, clearToken, flushPending, getDeviceFP, getToken, loadPending, pushPending, setToken } from '../api/client'
import { QuizSocket, wsURL } from '../ws/socket'

interface Question {
  id: number
  content: string
  options: string[]
  score: number
  difficulty: string
}

interface AnswerRes {
  correct: boolean
  correct_answer: number
  score: number
  duplicate: boolean
}

interface RoundResp {
  round_no: number
  questions: Question[]
  question_seconds: number
}

type Phase = 'name' | 'loading' | 'quiz' | 'done'

export default function Quiz() {
  const [phase, setPhase] = useState<Phase>('name')
  const [nickname, setNickname] = useState('')
  const [roundNo, setRoundNo] = useState(1)
  const [questions, setQuestions] = useState<Question[]>([])
  const [idx, setIdx] = useState(0)
  const [selected, setSelected] = useState<number | null>(null)
  const [feedback, setFeedback] = useState<AnswerRes | null>(null)
  const [seconds, setSeconds] = useState(20)
  const [qSeconds, setQSeconds] = useState(20)
  const [error, setError] = useState('')
  const [flash, setFlash] = useState('')
  const [myNick, setMyNick] = useState(() => sessionStorage.getItem('quiz_nick') || '')
  const [identityConflict, setIdentityConflict] = useState('')
  const [boundNick, setBoundNick] = useState<string | null>(null)
  const locked = useRef(false)
  const sockRef = useRef<QuizSocket | null>(null)

  // 建立带 token 的实时连接（在线人次统计/特效/公告）
  const connectSocket = () => {
    const t = getToken()
    if (!t) return
    sockRef.current?.close()
    sockRef.current = new QuizSocket(`${wsURL()}?token=${encodeURIComponent(t)}`, () => undefined)
    sockRef.current.connect()
  }

  useEffect(() => () => sockRef.current?.close(), [])

  // 本机是否已有绑定身份（未登录时查询，用于直接展示恢复选项）
  const checkBound = async () => {
    try {
      const d = await api<{ nickname: string }>(
        `/api/identity/current?device_fp=${encodeURIComponent(getDeviceFP())}`,
      )
      setBoundNick(d.nickname)
    } catch {
      setBoundNick('')
    }
  }

  useEffect(() => {
    if (phase === 'name' && !getToken()) void checkBound()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [phase])

  // 答题中离开页面提醒（关闭/刷新）
  useEffect(() => {
    if (phase !== 'quiz') return
    const h = (e: BeforeUnloadEvent) => {
      e.preventDefault()
      e.returnValue = ''
    }
    window.addEventListener('beforeunload', h)
    return () => window.removeEventListener('beforeunload', h)
  }, [phase])

  function enterRound(d: RoundResp, flashMsg: string) {
    setRoundNo(d.round_no)
    setQuestions(d.questions)
    setQSeconds(d.question_seconds)
    setSeconds(d.question_seconds)
    setIdx(0)
    setFeedback(null)
    setSelected(null)
    locked.current = false
    if (flashMsg) setFlash(flashMsg)
    if (d.questions.length === 0) {
      setError('当前难度下没有题目，请联系现场工作人员')
      setPhase('name')
      return
    }
    setPhase('quiz')
  }

  // 已有 token → 恢复进行中的一轮；本轮已答完则自动开新一轮
  useEffect(() => {
    if (phase !== 'name' || !getToken()) return
    void resume()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function resume() {
    setPhase('loading')
    try {
      connectSocket()
      const flushed = await flushPending()
      if (flushed > 0) setFlash(`已补交 ${flushed} 条缓存的答案`)
      void api<{ nickname: string }>('/api/me')
        .then((m) => {
          setMyNick(m.nickname)
          sessionStorage.setItem('quiz_nick', m.nickname)
        })
        .catch(() => undefined)
      try {
        const cur = await api<RoundResp & { answered: number }>('/api/quiz/current')
        const answered = Math.min(cur.answered, cur.questions.length)
        setRoundNo(cur.round_no)
        setQuestions(cur.questions)
        setQSeconds(cur.question_seconds)
        setSeconds(cur.question_seconds)
        if (answered < cur.questions.length) {
          setIdx(answered)
          setPhase('quiz')
          return
        }
      } catch {
        /* 无进行中轮次 → 下面开新轮 */
      }
      const d = await api<RoundResp>('/api/quiz/round', { method: 'POST' })
      enterRound(d, `新一轮答题开始（第 ${d.round_no} 轮），历史最佳成绩仍保留`)
    } catch (e) {
      clearToken()
      setError((e as Error).message)
      setPhase('name')
    }
  }

  async function start(nickOverride?: string) {
    const nick = nickOverride ?? nickname
    setError('')
    setPhase('loading')
    try {
      const s = await api<{ token: string; nickname: string; recovered?: boolean }>('/api/session', {
        method: 'POST',
        body: JSON.stringify({ nickname: nick, device_fp: getDeviceFP() }),
      })
      setToken(s.token)
      sessionStorage.setItem('quiz_nick', s.nickname || nickname)
      setMyNick(s.nickname || nickname)
      setIdentityConflict('')
      connectSocket()
      const flushed = await flushPending()
      if (flushed > 0) setFlash(`已补交 ${flushed} 条缓存的答案`)
      if (s.recovered) {
        setFlash('检测到本设备已有答题身份，已自动恢复')
      }
      const d = await api<RoundResp>('/api/quiz/round', { method: 'POST' })
      enterRound(d, '')
    } catch (e) {
      const err = e as Error & { code?: string }
      if (err.code === 'identity_bound') {
        setIdentityConflict(err.message)
      } else {
        setError(err.message)
      }
      setPhase('name')
    }
  }

  // 放弃当前身份：解除设备绑定，本机可重新注册
  const abandonIdentity = async (byDevice: boolean) => {
    try {
      if (byDevice) {
        await api('/api/identity/abandon', { method: 'POST', body: JSON.stringify({ device_fp: getDeviceFP() }) })
      } else {
        // 放弃自己的身份：必须携带本机指纹，服务端校验归属
        await api('/api/identity/abandon', {
          method: 'POST',
          body: JSON.stringify({ device_fp: getDeviceFP() }),
        })
      }
    } catch (e) {
      setError((e as Error).message)
      return
    }
    clearToken()
    sessionStorage.removeItem('quiz_nick')
    sockRef.current?.close()
    setMyNick('')
    setIdentityConflict('')
    setError('')
    setFlash('')
    setQuestions([])
    setIdx(0)
    setFeedback(null)
    setSelected(null)
    locked.current = false
    setPhase('name')
    void checkBound()
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
            第 {idx + 1} / {total} 题 · 第 {roundNo} 轮
            <span className={`diff-tag d-${q.difficulty}`}>{q.difficulty}</span>
            <span className="id-tag">答题身份：{myNick}</span>
          </span>
          <span className="quiz-head-right">
            <span className={`timer${seconds <= 5 ? ' danger' : ''}`}>{seconds}s</span>
            <button className="mini-quit" onClick={() => void abandonIdentity(false)}>
              放弃身份
            </button>
          </span>
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

  if (phase === 'loading') {
    return (
      <section className="card center">
        <p className="muted">加载中…</p>
      </section>
    )
  }

  return (
    <section className="card center">
      <h2>开始答题</h2>
      {boundNick ? (
        <div className="bound-box">
          <p>本机已有答题身份</p>
          <p className="bound-nick">「{boundNick}」</p>
          <div className="bound-actions">
            <button className="btn btn-primary" onClick={() => void start(boundNick)}>
              恢复此身份
            </button>
            <button className="btn" onClick={() => void abandonIdentity(true)}>
              放弃并注册新身份
            </button>
          </div>
        </div>
      ) : (
        <>
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
            <button className="btn btn-primary" type="submit">
              进入答题
            </button>
          </form>
        </>
      )}
      {identityConflict && (
        <div className="conflict-box">
          <p className="error">{identityConflict}</p>
          <button className="btn" onClick={() => void abandonIdentity(true)}>
            放弃原身份，重新注册
          </button>
        </div>
      )}
      {error && <p className="error">{error}</p>}
      {loadPending().length > 0 && getToken() === null && (
        <p className="muted">检测到 {loadPending().length} 条未提交答案，答题后将自动补交</p>
      )}
    </section>
  )
}
