import { lazy, Suspense, useCallback, useEffect, useRef, useState, type CSSProperties } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'

const ParticlesBg = lazy(() => import('../components/ParticlesBg'))

interface Motion {
  copies: number
  stripW: number
  dur: number
  scroll: boolean
  rows: number
}

export default function Home() {
  const [bgs, setBgs] = useState<string[]>([])
  const stripRef = useRef<HTMLDivElement>(null)
  const [motion, setMotion] = useState<Motion>({ copies: 1, stripW: 0, dur: 24, scroll: false, rows: 3 })

  // 拉取后台配置的背景图；没有则回退粒子动画
  useEffect(() => {
    void api<{ backgrounds: string[] }>('/api/backgrounds')
      .then((d) => setBgs((d.backgrounds || []).filter(Boolean)))
      .catch(() => undefined)
  }, [])

  // 多行照片带横向无缝滚动：带宽超过视口时滚动，否则静止；
  // 行数随设备自适应（桌面 3 行 / 手机 2 行），图片保持原比例。
  const calc = useCallback(() => {
    const el = stripRef.current
    if (!el || bgs.length === 0) return
    const stripW = el.scrollWidth
    const vw = window.innerWidth
    const rows = vw < 640 ? 2 : 3
    const scroll = stripW > vw
    const copies = scroll ? Math.ceil(vw / Math.max(stripW, 1)) + 2 : 1
    const dur = Math.max(stripW / 30, 24) // ~30px/s
    setMotion((m) =>
      m.stripW === stripW && m.scroll === scroll && m.rows === rows
        ? m
        : { copies, stripW, dur, scroll, rows },
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
    '--strip-w': `${motion.stripW}px`,
  } as CSSProperties

  return (
    <section className="home">
      {bgs.length > 0 ? (
        <div className="bg-marquee" aria-hidden="true">
          <div
            className={`bg-track-h${motion.scroll ? '' : ' static'}`}
            style={trackStyle}
          >
            {Array.from({ length: motion.copies }).map((_, c) => (
              <div
                key={c}
                className="bg-strip"
                ref={c === 0 ? stripRef : undefined}
                style={{ gridTemplateRows: `repeat(${motion.rows}, minmax(0, 1fr))` }}
              >
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
