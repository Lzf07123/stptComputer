import { lazy, Suspense, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'

const ParticlesBg = lazy(() => import('../components/ParticlesBg'))

export default function Home() {
  const [bgs, setBgs] = useState<string[]>([])
  const [bgIdx, setBgIdx] = useState(0)

  // 拉取后台配置的背景图；没有则回退粒子动画
  useEffect(() => {
    void api<{ backgrounds: string[] }>('/api/backgrounds')
      .then((d) => setBgs((d.backgrounds || []).filter(Boolean)))
      .catch(() => undefined)
  }, [])

  // 6s 轮换
  useEffect(() => {
    if (bgs.length <= 1) return
    const t = window.setInterval(() => setBgIdx((i) => (i + 1) % bgs.length), 6000)
    return () => clearInterval(t)
  }, [bgs.length])

  return (
    <section className="home">
      {bgs.length > 0 ? (
        <div className="bg-slideshow" aria-hidden="true">
          {bgs.map((u, i) => (
            <div
              key={u}
              className={`bg-slide${i === bgIdx ? ' on' : ''}`}
              style={{ backgroundImage: `url(${u})` }}
            />
          ))}
          <div className="bg-veil" />
        </div>
      ) : (
        <Suspense fallback={<div className="particles-fallback" />}>
          <ParticlesBg />
        </Suspense>
      )}
      <div className="home-inner">
        <p className="eyebrow fade-in">STPT-2026 · QUIZ CHALLENGE</p>
        <h1 className="fade-in d1">计算机协会</h1>
        <p className="subtitle fade-in d2">答题挑战 · 实时排行 · 现场大奖</p>
        <div className="home-actions fade-in d3">
          <Link className="btn btn-secondary" to="/join">加入群聊</Link>
          <Link className="btn btn-primary" to="/quiz">开始答题</Link>
        </div>
      </div>
    </section>
  )
}
