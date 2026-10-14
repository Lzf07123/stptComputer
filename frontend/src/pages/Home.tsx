import { lazy, Suspense, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'

const ParticlesBg = lazy(() => import('../components/ParticlesBg'))

export default function Home() {
  const [bgs, setBgs] = useState<string[]>([])

  // 拉取后台配置的背景图；没有则回退粒子动画
  useEffect(() => {
    void api<{ backgrounds: string[] }>('/api/backgrounds')
      .then((d) => setBgs((d.backgrounds || []).filter(Boolean)))
      .catch(() => undefined)
  }, [])

  // 滚动轮播：轨道放两份图片实现无缝循环；
  // 单张图静止；多张时按数量自适应（每张宽度至少占满 1/n 视口，速度随数量变慢）
  const panelW = bgs.length > 0 ? `max(55vw, ${100 / bgs.length}vw)` : '55vw'
  const duration = Math.max(bgs.length * 9, 24)

  return (
    <section className="home">
      {bgs.length > 0 ? (
        <div className="bg-marquee" aria-hidden="true">
          <div
            className={`bg-track${bgs.length === 1 ? ' static' : ''}`}
            style={{ animationDuration: `${duration}s` }}
          >
            {[...bgs, ...bgs].map((u, i) => (
              <div
                key={`${u}-${i}`}
                className="bg-panel"
                style={{ backgroundImage: `url(${u})`, width: panelW }}
              />
            ))}
          </div>
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
