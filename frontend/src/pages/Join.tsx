import { useEffect, useState } from 'react'
import { api } from '../api/client'

interface Qrcode {
  id: number
  title: string
  image_url: string
}

export default function Join() {
  const [qrs, setQrs] = useState<Qrcode[]>([])
  const [error, setError] = useState('')
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    let alive = true
    const load = async () => {
      try {
        const d = await api<{ qrcodes: Qrcode[] }>('/api/qrcode')
        if (alive) {
          setQrs(d.qrcodes || [])
          setError('')
          setLoaded(true)
        }
      } catch (e) {
        if (alive) {
          setError((e as Error).message)
          setLoaded(true)
        }
      }
    }
    void load()
    const t = window.setInterval(load, 30000) // 轮询：后台增删改后自动生效
    return () => {
      alive = false
      clearInterval(t)
    }
  }, [])

  return (
    <section className="card center">
      <h2>加入我们</h2>
      <p className="muted">扫码加入招新群，获取最新招新资讯</p>
      {qrs.length > 0 ? (
        <div className="qr-grid">
          {qrs.map((qr) => (
            <figure className="qr-item" key={qr.id}>
              <img className="qrcode-img" src={qr.image_url} alt={qr.title || '二维码'} />
              {qr.title && <figcaption className="qr-title">{qr.title}</figcaption>}
            </figure>
          ))}
        </div>
      ) : (
        <div className="qrcode-placeholder">
          {!loaded ? '加载中…' : error ? `加载失败：${error}` : '二维码暂未配置，请联系现场工作人员'}
        </div>
      )}
    </section>
  )
}
