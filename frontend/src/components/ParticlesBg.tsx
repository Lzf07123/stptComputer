import { useEffect, useState } from 'react'
import Particles, { initParticlesEngine } from '@tsparticles/react'
import { loadSlim } from '@tsparticles/slim'

export default function ParticlesBg() {
  const [ready, setReady] = useState(false)

  useEffect(() => {
    let alive = true
    initParticlesEngine(async (engine) => {
      await loadSlim(engine)
    }).then(() => {
      if (alive) setReady(true)
    })
    return () => {
      alive = false
    }
  }, [])

  if (!ready) return <div className="particles-fallback" />

  return (
    <Particles
      id="tsparticles"
      options={{
        background: { color: { value: 'transparent' } },
        fpsLimit: 60,
        detectRetina: true,
        particles: {
          number: { value: 80, density: { enable: true } },
          color: { value: '#22d3ee' },
          links: { enable: true, color: '#155e75', distance: 140 },
          move: { enable: true, speed: 1.2 },
          opacity: { value: 0.5 },
          size: { value: 2 },
        },
      }}
    />
  )
}
