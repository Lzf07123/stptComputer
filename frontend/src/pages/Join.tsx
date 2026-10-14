import { useEffect, useState } from 'react'
import { api } from '../api/client'

export default function Join() {
  const [imageUrl, setImageUrl] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    let alive = true
    const load = async () => {
      try {
        const d = await api<{ image_url: string }>('/api/qrcode')
        if (alive) {
          setImageUrl(d.image_url)
          setError('')
        }
      } catch (e) {
        if (alive) setError((e as Error).message)
      }
    }
    void load()
    const t = window.setInterval(load, 30000) // 活码轮询，后台换群自动生效
    return () => {
      alive = false
      clearInterval(t)
    }
  }, [])

  return (
    <section className="card center">
      <h2>加入计算机协会</h2>
      <p className="muted">扫码加入招新群，群满后二维码会自动切换</p>
      {imageUrl ? (
        <img className="qrcode-img" src={imageUrl} alt="群二维码" />
      ) : (
        <div className="qrcode-placeholder">
          {error ? `加载失败：${error}` : '二维码暂未配置，请联系现场工作人员'}
        </div>
      )}
    </section>
  )
}
