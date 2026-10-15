import { useEffect, useState } from 'react'
import { api } from './client'

// 将站点 Logo 应用到浏览器标签页图标；传空字符串时移除（恢复默认）。
export function setFavicon(url: string) {
  const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!url) {
    if (link && link.dataset.auto === '1') link.remove()
    return
  }
  const el = link ?? document.createElement('link')
  el.rel = 'icon'
  el.dataset.auto = '1'
  el.href = url
  if (!el.parentNode) document.head.appendChild(el)
}

// 站点级配置（Logo 等）：随页面加载拉取；未配置 Logo 时返回空字符串（使用内置样式）。
export function useLogoURL(): string {
  const [url, setUrl] = useState('')
  useEffect(() => {
    let alive = true
    void api<{ logo_url?: string }>('/api/config')
      .then((d) => {
        if (alive) {
          setUrl(d.logo_url || '')
          setFavicon(d.logo_url || '')
        }
      })
      .catch(() => undefined)
    return () => {
      alive = false
    }
  }, [])
  return url
}
