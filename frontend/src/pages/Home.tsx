import { lazy, Suspense, useCallback, useEffect, useRef, useState, type CSSProperties } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'

const ParticlesBg = lazy(() => import('../components/ParticlesBg'))

interface Motion {
  copies: number
  gridH: number
  dur: number
  scroll: boolean
}

export default function Home() {
  const [bgs, setBgs] = useState<string[]>([])
  const gridRef = useRef<HTMLDivElement>(null)
  const [motion, setMotion] = useState<Motion>({ copies: 1, gridH: 0, dur: 24, scroll: false })

  // 拉取后台配置的背景图；没有则回退粒子动画
  useEffect(() => {
    void api<{ backgrounds: string[] }>('/api/backgrounds')
      .then((d) => setBgs((d.backgrounds || []).filter(Boolean)))
      .catch(() => undefined)
  }, [])

  // 按原比例多行照片墙：内容高于一屏时纵向无缝滚动；
  // 不超过一屏时静止展示（图少自适应）。
  const calc = useCallback(() => {
    const el = gridRef.current
    if (!el || bgs.length === 0) return
    const gridH = el.offsetHeight
    const vh = window.innerHeight
    const scroll = gridH > vh
    const copies = scroll ? Math.ceil(vh / Math.max(gridH, 1)) + 2 : 1
    const dur = Math.max(gridH / 26, 24) // ~26px/s
    setMotion((m) =>
      m.gridH === gridH && m.scroll === scroll ? m : { copies, gridH, dur, scroll },
    )
  }, [bgs])

  useEffect(() => {
    calc()
    const onResize = () => calc()
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [calc])

  const trackStyle = {
    animationDuration: `${motion.dur}s`,
    '--grid-h': `${motion.gridH}px`,
  } as CSSProperties

  return (
    <section className="home">
      {bgs.length > 0 ? (
        <div className="bg-marquee" aria-hidden="true">
          <div
            className={`bg-track-v${motion.scroll ? '' : ' static'}`}
            style={trackStyle}
          >
            {Array.from({ length: motion.copies }).map((_, c) => (
              <div key={c} className="bg-grid" ref={c === 0 ? gridRef : undefined}>
                {bgs.map((u) => (
                  <img
                    key={`${u}-${c}`}
                    src={u}
                    alt=""
                    loading={c === 0 ? undefined : 'lazy'}
                    onLoad={c === 0 ? calc : undefined}
                  />
                ))}
              </div>
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
