import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Navigate, useBlocker, useNavigate } from 'react-router-dom'
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
  const [needsNewRound, setNeedsNewRound] = useState(false)
  const [exitedInfo, setExitedInfo] = useState<{ round_no: number; answered: number; total: number } | null>(null)
  const [difficulty, setDifficulty] = useState('全部')
  const [confirmReq, setConfirmReq] = useState<{ kind: 'identity' | 'round'; byDevice: boolean; step: 'ask' | 'final' } | null>(null)
  const locked = useRef(false)
  const sockRef = useRef<QuizSocket | null>(null)
  const nav = useNavigate()
  const allowLeaveRef = useRef(false)

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

  // 答题中离开页面提醒：
  // 1) 关闭/刷新标签 → beforeunload
  useEffect(() => {
    if (phase !== 'quiz') return
    const h = (e: BeforeUnloadEvent) => {
      e.preventDefault()
      e.returnValue = ''
    }
    window.addEventListener('beforeunload', h)
    return () => window.removeEventListener('beforeunload', h)
  }, [phase])

  // 2) 页内跳转 / 浏览器前进后退 → useBlocker 确认
  const blocker = useBlocker(phase === 'quiz')
  useEffect(() => {
    if (blocker.state !== 'blocked') return
    // 「退出答题」按钮主动退出：跳过确认
    if (allowLeaveRef.current) {
      allowLeaveRef.current = false
      blocker.proceed()
      return
    }
    if (window.confirm('答题进行中，确定要离开本页吗？')) {
      blocker.proceed()
    } else {
      blocker.reset()
    }
  }, [blocker])

  // 退出答题：保留身份与进度，返回首页；标记已退出，刷新/再进不自动恢复
  const exitQuiz = () => {
    sessionStorage.setItem('quiz_exited', '1')
    allowLeaveRef.current = true
    nav('/')
  }

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

  // 已有 token → 恢复进行中的一轮；但「退出答题」后不自动恢复，停留在选择界面
  useEffect(() => {
    if (phase !== 'name' || !getToken()) return
    if (sessionStorage.getItem('quiz_exited') === '1') {
      void checkExitedRound()
      return
    }
    void resume()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // 身份失效（token 401）：静默清理并回到本机身份界面
  const authLost = () => {
    clearToken()
    sessionStorage.removeItem('quiz_nick')
    setMyNick('')
    setNeedsNewRound(false)
    sockRef.current?.close()
    setFlash('原答题身份已失效，请重新进入')
    setError('')
    setPhase('name')
    void checkBound()
  }

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
          sessionStorage.removeItem('quiz_exited')
          setIdx(answered)
          setPhase('quiz')
          return
        }
      } catch (e) {
        if ((e as Error & { status?: number }).status === 401) {
          authLost()
          return
        }
        /* 无进行中轮次 → 选难度开新轮 */
      }
      setNeedsNewRound(true)
      setPhase('name')
    } catch (e) {
      const err = e as Error & { status?: number }
      if (err.status === 401) {
        authLost()
        return
      }
      // 非身份问题（网络/服务器瞬时错误）：保留 token，提示可重试
      setError(`${err.message}（可刷新重试）`)
      setPhase('name')
    }
  }

  // 退出答题后：查询进行中的本轮，展示「已退出答题」选择界面
  const checkExitedRound = async () => {
    try {
      const cur = await api<RoundResp & { answered: number }>('/api/quiz/current')
      setExitedInfo({
        round_no: cur.round_no,
        answered: Math.min(cur.answered, cur.questions.length),
        total: cur.questions.length,
      })
      setPhase('name')
    } catch {
      setExitedInfo(null)
      setNeedsNewRound(true)
      setPhase('name')
    }
  }

  // 继续本轮：恢复进度进入答题
  const continueRound = async () => {
    sessionStorage.removeItem('quiz_exited')
    setPhase('loading')
    try {
      const cur = await api<RoundResp & { answered: number }>('/api/quiz/current')
      const answered = Math.min(cur.answered, cur.questions.length)
      setRoundNo(cur.round_no)
      setQuestions(cur.questions)
      setQSeconds(cur.question_seconds)
      setSeconds(cur.question_seconds)
      if (answered >= cur.questions.length) {
        setNeedsNewRound(true)
        setPhase('name')
        return
      }
      connectSocket()
      setIdx(answered)
      setExitedInfo(null)
      setPhase('quiz')
    } catch {
      setNeedsNewRound(true)
      setPhase('name')
    }
  }

  // 按所选难度开始新一轮
  async function startNewRound() {
    sessionStorage.removeItem('quiz_exited')
    setError('')
    setPhase('loading')
    try {
      const d = await api<RoundResp>('/api/quiz/round', {
        method: 'POST',
        body: JSON.stringify({ difficulty }),
      })
      enterRound(d, '')
    } catch (e) {
      setError((e as Error).message)
      setPhase('name')
    }
  }

  async function start(nickOverride?: string) {
    const nick = nickOverride ?? nickname
    sessionStorage.removeItem('quiz_exited')
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
      const d = await api<RoundResp>('/api/quiz/round', {
        method: 'POST',
        body: JSON.stringify({ difficulty }),
      })
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

  // 二次确认入口
  const requestAbandon = (byDevice: boolean) => setConfirmReq({ kind: 'identity', byDevice, step: 'ask' })
  const requestAbandonRound = () => setConfirmReq({ kind: 'round', byDevice: false, step: 'ask' })

  const confirmAbandon = async () => {
    if (!confirmReq) return
    const req = confirmReq
    setConfirmReq(null)
    if (req.kind === 'identity') {
      await abandonIdentity(req.byDevice)
      return
    }
    // 放弃本轮：作废本轮成绩，身份与历史最佳保留
    try {
      await api('/api/quiz/round/abandon', { method: 'POST' })
      sessionStorage.removeItem('quiz_exited')
      setExitedInfo(null)
      sockRef.current?.close()
      setQuestions([])
      setIdx(0)
      setFeedback(null)
      setSelected(null)
      locked.current = false
      setFlash('本轮已放弃，可重新选择难度开始新一轮')
      setError('')
      setNeedsNewRound(true)
      setPhase('name')
    } catch (e) {
      setError((e as Error).message)
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
    sessionStorage.removeItem('quiz_exited')
    sockRef.current?.close()
    setMyNick('')
    setIdentityConflict('')
    setError('')
    setFlash('')
    setQuestions([])
    setIdx(0)
    setFeedback(null)
    setSelected(null)
    setNeedsNewRound(false)
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
      // 答题结束跳转成绩页：主动放行，不弹离开确认
      allowLeaveRef.current = true
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
  const total = questions.length

  const diffPicker = (
    <div className="diff-picker">
      {['全部', '容易', '简单', '困难'].map((d) => (
        <button
          key={d}
          type="button"
          className={`diff-opt${difficulty === d ? ' on' : ''}`}
          onClick={() => setDifficulty(d)}
        >
          {d}
        </button>
      ))}
    </div>
  )

  let body: ReactNode
  if (phase === 'quiz' && q) {
    body = (
      <section className="quiz-wrap">
        <div className="quiz-head">
          <span className="muted">
            第 {idx + 1} / {total} 题 · 第 {roundNo} 轮
            <span className={`diff-tag d-${q.difficulty}`}>{q.difficulty}</span>
            <span className="id-tag">答题身份：{myNick}</span>
          </span>
          <span className="quiz-head-right">
            <span className={`timer${seconds <= 5 ? ' danger' : ''}`}>{seconds}s</span>
            <button className="mini-exit" onClick={exitQuiz} title="保留身份与进度，返回首页">
              退出答题
            </button>
            <button className="mini-quit" onClick={requestAbandonRound}>
              放弃本轮
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
                <span className="opt-text">{opt}</span>
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
  } else if (phase === 'loading') {
    body = (
      <section className="card center">
        <p className="muted">加载中…</p>
      </section>
    )
  } else if (exitedInfo) {
    body = (
      <section className="card center">
        <h2>已退出答题</h2>
        <p className="muted">
          第 {exitedInfo.round_no} 轮进度已保留：已答 {exitedInfo.answered} / {exitedInfo.total} 题
        </p>
        <div className="bound-actions">
          <button className="btn btn-primary" onClick={() => void continueRound()}>
            继续本轮
          </button>
          <button className="btn" onClick={requestAbandonRound}>
            放弃本轮
          </button>
        </div>
        <p className="newround-id" style={{ marginTop: 14 }}>
          <span className="id-tag">当前身份：{myNick}</span>
          <button className="mini-quit" onClick={() => requestAbandon(false)}>
            放弃当前身份
          </button>
        </p>
        {flash && <p className="flash">{flash}</p>}
        {error && <p className="error">{error}</p>}
      </section>
    )
  } else if (needsNewRound) {
    body = (
      <section className="card center">
        <h2>新一轮答题</h2>
        <p className="muted">答题前请选择本局难度（历史最佳成绩仍保留）</p>
        {diffPicker}
        <button className="btn btn-primary" style={{ marginTop: 16 }} onClick={() => void startNewRound()}>
          开始本局答题
        </button>
        <p className="newround-id" style={{ marginTop: 14 }}>
          <span className="id-tag">当前身份：{myNick}</span>
          <button className="mini-quit" onClick={() => requestAbandon(false)}>
            放弃当前身份
          </button>
        </p>
        {flash && <p className="flash">{flash}</p>}
        {error && <p className="error">{error}</p>}
      </section>
    )
  } else {
    body = (
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
              <button className="btn" onClick={() => requestAbandon(true)}>
                放弃并注册新身份
              </button>
            </div>
          </div>
        ) : (
          <>
            <p className="muted">输入昵称领取你的答题身份（每人限领一次）</p>
            {diffPicker}
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
            <button className="btn" onClick={() => requestAbandon(true)}>
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

  return (
    <>
      {body}
      {confirmReq && (
        <div className="modal-veil" onClick={() => setConfirmReq(null)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            {confirmReq.step === 'ask' ? (
              <>
                <h3>{confirmReq.kind === 'identity' ? '放弃当前答题身份？' : '放弃本轮答题？'}</h3>
                <ul className="side-effects">
                  {confirmReq.kind === 'identity' ? (
                    <>
                      <li>身份将<b>立即失效</b>，无法再登录或继续答题</li>
                      <li>昵称「{myNick || boundNick || '当前身份'}」将<b>永久封存</b>，任何人（含本人）无法再次使用</li>
                      <li>排行榜中的<b>历史记录保留</b>，将作为足迹继续展示</li>
                      <li>历史答题保留，参与人次统计不受影响</li>
                      <li>本机解除绑定，可重新注册<b>全新</b>身份</li>
                    </>
                  ) : (
                    <>
                      <li>本轮已提交的答案将<b>全部作废</b>（不含历史轮次）</li>
                      <li>身份与<b>历史最佳成绩保留</b>，排行榜不受影响</li>
                      <li>放弃后可重新选择难度开始新一轮</li>
                    </>
                  )}
                </ul>
                <div className="modal-actions">
                  <button className="btn" onClick={() => setConfirmReq(null)}>取消</button>
                  <button
                    className="btn btn-primary"
                    onClick={() => setConfirmReq({ ...confirmReq, step: 'final' })}
                  >
                    继续
                  </button>
                </div>
              </>
            ) : (
              <>
                <h3>最后确认</h3>
                <p className="muted">
                  {confirmReq.kind === 'identity'
                    ? `放弃操作不可撤销：身份立即失效，昵称「${myNick || boundNick || '当前身份'}」永久不可再用。确定要放弃吗？`
                    : '确认放弃本轮？本轮已提交的答案将被作废（历史最佳成绩不受影响）。'}
                </p>
                <div className="modal-actions">
                  <button className="btn" onClick={() => setConfirmReq({ ...confirmReq, step: 'ask' })}>
                    返回
                  </button>
                  <button className="btn btn-primary" onClick={() => void confirmAbandon()}>
                    确认放弃
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      )}
    </>
  )
}
