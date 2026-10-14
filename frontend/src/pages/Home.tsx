import { lazy, Suspense } from 'react'
import { Link } from 'react-router-dom'

const ParticlesBg = lazy(() => import('../components/ParticlesBg'))

export default function Home() {
  return (
    <section className="home">
      <Suspense fallback={<div className="particles-fallback" />}>
        <ParticlesBg />
      </Suspense>
      <div className="home-inner">
        <p className="eyebrow fade-in">STPT · 2026 招新季</p>
        <h1 className="fade-in d1">计算机协会</h1>
        <p className="subtitle fade-in d2">答题挑战 · 实时排行 · 现场大奖</p>
        <div className="home-actions fade-in d3">
          <Link className="btn btn-primary" to="/join">加入群聊</Link>
          <Link className="btn btn-accent" to="/quiz">开始答题</Link>
        </div>
      </div>
    </section>
  )
}
