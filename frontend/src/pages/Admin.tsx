import { useCallback, useEffect, useState, type ChangeEvent } from 'react'
import { api } from '../api/client'

interface Bg {
  id: number
  file_path: string
  active: boolean
}

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
  const [bgs, setBgs] = useState<Bg[]>([])
  const [uploading, setUploading] = useState(false)

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

  const reloadBgs = useCallback(async () => {
    try {
      const d = await api<{ backgrounds: Bg[] }>('/api/admin/backgrounds', { headers: adminHeaders() })
      setBgs(d.backgrounds || [])
    } catch {
      /* 忽略 */
    }
  }, [adminHeaders])

  useEffect(() => {
    if (pass) {
      void reload()
      void reloadBgs()
    }
  }, [pass, reload, reloadBgs])

  const uploadOne = async (f: File): Promise<void> => {
    const fd = new FormData()
    fd.append('file', f)
    const res = await fetch('/api/admin/backgrounds', {
      method: 'POST',
      headers: { 'X-Admin-Token': pass },
      body: fd,
    })
    if (!res.ok) {
      const b = await res.json().catch(() => ({ error: `HTTP ${res.status}` }))
      const msg = b.error === `HTTP ${res.status}` && res.status === 413
        ? '图片超过服务器大小限制，请压缩到 10MB 以内'
        : (b.error || `HTTP ${res.status}`)
      throw new Error(`${f.name}：${msg}`)
    }
  }

  // 批量上传：最多 3 个并发，逐个提交，统计成功/失败
  const onPickBg = async (e: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || [])
    e.target.value = ''
    if (!files.length || !pass) return
    setUploading(true)
    let ok = 0
    const errors: string[] = []
    const queue = [...files]
    const worker = async () => {
      while (queue.length) {
        const f = queue.shift()
        if (!f) return
        try {
          await uploadOne(f)
          ok++
        } catch (err) {
          errors.push((err as Error).message)
        }
      }
    }
    await Promise.all(Array.from({ length: Math.min(3, files.length) }, worker))
    setUploading(false)
    if (errors.length) {
      setMsg(`✘ 上传完成：成功 ${ok}/${files.length} 张；${errors.join('；')}`)
    } else {
      setMsg(`✔ 批量上传成功：${ok} 张，首页已生效`)
    }
    void reloadBgs()
  }

  const toggleBg = (b: Bg) =>
    void act(`/api/admin/backgrounds/${b.id}/active`, { active: !b.active }, b.active ? '背景已停用' : '背景已启用')

  const delBg = async (b: Bg) => {
    try {
      await api(`/api/admin/backgrounds/${b.id}`, { method: 'DELETE', headers: adminHeaders() })
      setMsg('✔ 背景已删除')
      void reloadBgs()
    } catch (e) {
      setMsg(`✘ ${(e as Error).message}`)
    }
  }

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

          <div className="admin-panel">
            <h3>首页背景轮播</h3>
            <label className="field">
              {uploading ? '上传中…' : '批量上传图片（可多选，单张 ≤10MB）'}
              <input
                type="file"
                accept="image/*"
                multiple
                disabled={uploading}
                onChange={(e) => void onPickBg(e)}
              />
            </label>
            <ul className="qlist">
              {bgs.map((b) => (
                <li key={b.id}>
                  <span className={b.active ? '' : 'muted'}>
                    {b.active ? '▶ ' : ''}
                    {b.file_path.split('/').pop()}
                  </span>
                  <span>
                    <button className="mini-btn" onClick={() => void toggleBg(b)}>
                      {b.active ? '停用' : '启用'}
                    </button>
                    <button className="mini-btn" onClick={() => void delBg(b)}>
                      删除
                    </button>
                  </span>
                </li>
              ))}
              {bgs.length === 0 && <li className="muted">暂无背景图，未上传时显示粒子动画</li>}
            </ul>
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
