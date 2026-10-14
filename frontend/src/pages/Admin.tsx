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
  difficulty: string
}

interface OvResp {
  overview: {
    users: number
    attempts: number
    participants: number
    online: number
    questions: number
    backgrounds: number
    db: string
    redis: string
  }
  mem_mb: number
  goroutines: number
  uptime: string
}

export default function Admin() {
  const [pass, setPass] = useState(() => sessionStorage.getItem('admin_pass') || '')
  const [verified, setVerified] = useState(false)
  const [ov, setOv] = useState<OvResp | null>(null)
  const [msg, setMsg] = useState('')
  const [questions, setQuestions] = useState<Question[]>([])
  const [content, setContent] = useState('')
  const [options, setOptions] = useState('')
  const [answerIdx, setAnswerIdx] = useState(0)
  const [score, setScore] = useState(10)
  const [difficulty, setDifficulty] = useState('简单')
  const [bgs, setBgs] = useState<Bg[]>([])
  const [uploading, setUploading] = useState(false)
  const [footer, setFooter] = useState({ owner: '', license: '', icp: '', icp_icon: '', police: '', police_icon: '' })
  const [qrNow, setQrNow] = useState('')

  const adminHeaders = useCallback(
    () => ({ 'X-Admin-Token': pass }),
    [pass],
  )

  const verify = useCallback(async (p: string) => {
    try {
      const d = await api<OvResp>('/api/admin/overview', { headers: { 'X-Admin-Token': p } })
      setOv(d)
      setVerified(true)
      setMsg('')
      return true
    } catch (e) {
      setVerified(false)
      setOv(null)
      setMsg(`✘ ${(e as Error).message}`)
      return false
    }
  }, [])

  useEffect(() => {
    const p = sessionStorage.getItem('admin_pass')
    if (p) void verify(p)
  }, [verify])

  const submitPass = () => {
    sessionStorage.setItem('admin_pass', pass)
    void verify(pass)
  }

  // 概览 10s 自动刷新
  useEffect(() => {
    if (!verified || !pass) return
    const t = window.setInterval(() => {
      void api<OvResp>('/api/admin/overview', { headers: adminHeaders() })
        .then(setOv)
        .catch(() => undefined)
    }, 10000)
    return () => clearInterval(t)
  }, [verified, pass, adminHeaders])

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

  const reloadQr = useCallback(async () => {
    try {
      const d = await api<{ image_url: string }>('/api/qrcode')
      setQrNow(d.image_url || '')
    } catch {
      /* 忽略 */
    }
  }, [])

  const reloadFooter = useCallback(async () => {
    try {
      const d = await api<{ footer: { owner: string; license: string; icp: string; icp_icon: string; police: string; police_icon: string } }>('/api/admin/footer', { headers: adminHeaders() })
      setFooter(d.footer)
    } catch {
      /* 忽略 */
    }
  }, [adminHeaders])

  useEffect(() => {
    if (pass && verified) {
      void reload()
      void reloadBgs()
      void reloadFooter()
      void reloadQr()
    }
  }, [pass, verified, reload, reloadBgs, reloadFooter, reloadQr])

  // 上传到 /uploads 并返回 URL（群二维码、备案图标等）
  const uploadImage = async (f: File): Promise<string> => {
    const fd = new FormData()
    fd.append('file', f)
    const res = await fetch('/api/admin/uploads', {
      method: 'POST',
      headers: { 'X-Admin-Token': pass },
      body: fd,
    })
    if (!res.ok) {
      const b = await res.json().catch(() => ({ error: `HTTP ${res.status}` }))
      throw new Error(b.error || `HTTP ${res.status}`)
    }
    const d = await res.json()
    return d.url as string
  }

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

  if (!verified) {
    return (
      <section className="admin">
        <div className="card center">
          <h2>后台管理</h2>
          <p className="muted">请输入后台口令进行验证</p>
          <form
            className="name-form"
            onSubmit={(e) => {
              e.preventDefault()
              submitPass()
            }}
          >
            <input
              type="password"
              value={pass}
              onChange={(e) => setPass(e.target.value)}
              placeholder="后台口令"
              autoFocus
            />
            <button className="btn btn-primary" type="submit">
              验证
            </button>
          </form>
          {msg && <p className="error">{msg}</p>}
        </div>
      </section>
    )
  }

  return (
    <section className="admin">
      <div className="card">
        <h2>后台管理</h2>
        <p className="muted">
          已验证通过
          <button
            className="mini-btn"
            style={{ marginLeft: 8 }}
            onClick={() => {
              sessionStorage.removeItem('admin_pass')
              setVerified(false)
              setOv(null)
              setPass('')
            }}
          >
            退出验证
          </button>
        </p>
        {msg && <p className={`flash${msg.startsWith('✔') ? '' : ' error'}`}>{msg}</p>}

        {ov && (
          <div className="ov-grid">
            <div className="ov-item"><b>{ov.overview.users}</b><span>注册用户</span></div>
            <div className="ov-item"><b>{ov.overview.participants}</b><span>参与答题</span></div>
            <div className="ov-item"><b>{ov.overview.attempts}</b><span>累计答题</span></div>
            <div className="ov-item"><b>{ov.overview.online}</b><span>当前在线</span></div>
            <div className="ov-item"><b>{ov.overview.questions}</b><span>启用题目</span></div>
            <div className="ov-item"><b>{ov.overview.backgrounds}</b><span>背景图片</span></div>
            <div className="ov-item"><b>{ov.mem_mb} MB</b><span>内存占用</span></div>
            <div className="ov-item"><b>{ov.goroutines}</b><span>协程数</span></div>
            <div className="ov-item"><b>{ov.uptime}</b><span>运行时长</span></div>
            <div className={`ov-item ${ov.overview.db === 'ok' ? 'ok' : 'bad'}`}><b>{ov.overview.db}</b><span>数据库</span></div>
            <div className={`ov-item ${ov.overview.redis === 'ok' ? 'ok' : 'bad'}`}><b>{ov.overview.redis}</b><span>Redis</span></div>
          </div>
        )}

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
              上传新二维码图片（替换当前展示）
              <input
                type="file"
                accept="image/*"
                onChange={async (e) => {
                  const f = e.target.files && e.target.files[0]
                  if (!f) return
                  try {
                    const url = await uploadImage(f)
                    await act('/api/admin/qrcode', { image_url: url }, '已切换群二维码')
                    void reloadQr()
                  } catch (err) {
                    setMsg(`✘ ${(err as Error).message}`)
                  }
                }}
              />
            </label>
            {qrNow ? (
              <img className="thumb big" src={qrNow} alt="当前群二维码" />
            ) : (
              <p className="muted">尚未上传群二维码</p>
            )}
          </div>

          <div className="admin-panel">
            <h3>页脚信息</h3>
            <label className="field">
              拥有人 / 版权
              <input
                value={footer.owner}
                onChange={(e) => setFooter({ ...footer, owner: e.target.value })}
                placeholder="计算机协会 · 汕头职业技术学院"
              />
            </label>
            <label className="field">
              开源协议
              <input
                value={footer.license}
                onChange={(e) => setFooter({ ...footer, license: e.target.value })}
                placeholder="开源协议：MIT"
              />
            </label>
            <label className="field">
              ICP 备案号（留空显示「待填写」）
              <input
                value={footer.icp}
                onChange={(e) => setFooter({ ...footer, icp: e.target.value })}
                placeholder="粤ICP备XXXXXXXX号"
              />
            </label>
            <label className="field">
              ICP 备案图标（后台上传）
              <input
                type="file"
                accept="image/*"
                onChange={async (e) => {
                  const f = e.target.files && e.target.files[0]
                  if (!f) return
                  try {
                    const url = await uploadImage(f)
                    setFooter({ ...footer, icp_icon: url })
                    setMsg('✔ ICP 图标已上传，记得点保存')
                  } catch (err) {
                    setMsg(`✘ ${(err as Error).message}`)
                  }
                }}
              />
              {footer.icp_icon && <img className="thumb" src={footer.icp_icon} alt="ICP 图标预览" />}
            </label>
            <label className="field">
              公安备案号（如：粤公网安备 4405XXXXXXXXXX号）
              <input
                value={footer.police}
                onChange={(e) => setFooter({ ...footer, police: e.target.value })}
                placeholder="粤公网安备 XXXXXXXXXXXX号"
              />
            </label>
            <label className="field">
              公安备案图标（后台上传）
              <input
                type="file"
                accept="image/*"
                onChange={async (e) => {
                  const f = e.target.files && e.target.files[0]
                  if (!f) return
                  try {
                    const url = await uploadImage(f)
                    setFooter({ ...footer, police_icon: url })
                    setMsg('✔ 公安图标已上传，记得点保存')
                  } catch (err) {
                    setMsg(`✘ ${(err as Error).message}`)
                  }
                }}
              />
              {footer.police_icon && <img className="thumb" src={footer.police_icon} alt="公安图标预览" />}
            </label>
            <button className="btn btn-accent" onClick={() => void act('/api/admin/footer', footer, '页脚信息已更新')}>
              保存页脚
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
            <label className="field">
              难度
              <select value={difficulty} onChange={(e) => setDifficulty(e.target.value)}>
                {['容易', '简单', '困难'].map((d) => (
                  <option key={d} value={d}>{d}</option>
                ))}
              </select>
            </label>
            <button
              className="btn btn-primary"
              onClick={() =>
                void act(
                  '/api/admin/questions',
                  { content, options: options.split('\n').map((s) => s.trim()).filter(Boolean), answer: answerIdx, score, difficulty },
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
                    <i className={`qdiff d-${q.difficulty}`}>{q.difficulty}</i> #{q.id} {q.content.slice(0, 16)}
                    {q.content.length > 16 ? '…' : ''}
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
