import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'

interface Question {
  id: number
  content: string
  options: string[]
  answer: number
  score: number
  active: boolean
}

export default function Admin() {
  const [pass, setPass] = useState(() => sessionStorage.getItem('admin_pass') || '')
  const [msg, setMsg] = useState('')
  const [questions, setQuestions] = useState<Question[]>([])
  const [content, setContent] = useState('')
  const [options, setOptions] = useState('')
  const [answerIdx, setAnswerIdx] = useState(0)
  const [score, setScore] = useState(10)
  const [qrUrl, setQrUrl] = useState('')

  const adminHeaders = useCallback(
    () => ({ 'X-Admin-Token': pass }),
    [pass],
  )

  const reload = useCallback(async () => {
    try {
      const d = await api<{ questions: Question[] }>('/api/admin/questions', { headers: adminHeaders() })
      setQuestions(d.questions)
    } catch {
      /* 口令错误等，忽略 */
    }
  }, [adminHeaders])

  useEffect(() => {
    if (pass) void reload()
  }, [pass, reload])

  async function act(path: string, body: any, okMsg: string) {
    try {
      await api(path, { method: 'POST', headers: adminHeaders(), body: JSON.stringify(body) })
      setMsg(`✔ ${okMsg}`)
      void reload()
    } catch (e) {
      setMsg(`✘ ${(e as Error).message}`)
    }
  }

  return (
    <section className="admin">
      <div className="card">
        <h2>后台管理</h2>
        <label className="field">
          后台口令
          <input
            type="password"
            value={pass}
            onChange={(e) => {
              setPass(e.target.value)
              sessionStorage.setItem('admin_pass', e.target.value)
            }}
          />
        </label>
        {msg && <p className={`flash${msg.startsWith('✔') ? '' : ' error'}`}>{msg}</p>}

        <div className="admin-grid">
          <div className="admin-panel">
            <h3>答题控制</h3>
            <button className="btn btn-accent" onClick={() => void act('/api/admin/quiz-state', { state: 'start' }, '答题已开始')}>
              开始答题
            </button>
            <button className="btn" onClick={() => void act('/api/admin/quiz-state', { state: 'stop' }, '答题已结束')}>
              结束答题
            </button>
          </div>

          <div className="admin-panel">
            <h3>特效</h3>
            <button className="btn" onClick={() => void act('/api/admin/effect', { kind: 'confetti' }, '已触发彩蛋特效')}>
              🎉 全屏彩蛋
            </button>
          </div>

          <div className="admin-panel">
            <h3>公告</h3>
            <button
              className="btn"
              onClick={() => void act('/api/admin/announce', { content: prompt('公告内容') || '欢迎加入计算机协会！', type: 'info' }, '公告已广播')}
            >
              📢 广播公告
            </button>
          </div>

          <div className="admin-panel">
            <h3>群二维码</h3>
            <label className="field">
              新二维码图片 URL
              <input value={qrUrl} onChange={(e) => setQrUrl(e.target.value)} placeholder="https://…/qrcode.png" />
            </label>
            <button className="btn" onClick={() => void act('/api/admin/qrcode', { image_url: qrUrl }, '已切换二维码')}>
              切换活码
            </button>
          </div>
        </div>
      </div>

      <div className="card">
        <h3>题库（{questions.length}）</h3>
        <div className="admin-grid">
          <div className="admin-panel">
            <h3>新增题目</h3>
            <label className="field">
              题目
              <input value={content} onChange={(e) => setContent(e.target.value)} />
            </label>
            <label className="field">
              选项（每行一个）
              <textarea rows={4} value={options} onChange={(e) => setOptions(e.target.value)} />
            </label>
            <label className="field">
              正确选项序号（0 起）
              <input
                type="number"
                min={0}
                value={answerIdx}
                onChange={(e) => setAnswerIdx(Number(e.target.value))}
              />
            </label>
            <label className="field">
              分值
              <input type="number" min={1} value={score} onChange={(e) => setScore(Number(e.target.value))} />
            </label>
            <button
              className="btn btn-primary"
              onClick={() =>
                void act(
                  '/api/admin/questions',
                  { content, options: options.split('\n').map((s) => s.trim()).filter(Boolean), answer: answerIdx, score },
                  '题目已添加',
                )
              }
            >
              添加题目
            </button>
          </div>

          <div className="admin-panel">
            <h3>题目列表</h3>
            <ul className="qlist">
              {questions.map((q) => (
                <li key={q.id}>
                  <span className={q.active ? '' : 'muted'}>
                    #{q.id} {q.content.slice(0, 18)}
                    {q.content.length > 18 ? '…' : ''}
                  </span>
                  <button
                    className="mini-btn"
                    onClick={() => void act(`/api/admin/questions/${q.id}/active`, { active: !q.active }, q.active ? '已停用' : '已启用')}
                  >
                    {q.active ? '停用' : '启用'}
                  </button>
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    </section>
  )
}
