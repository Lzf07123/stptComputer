import { useEffect, useState } from 'react'
import { api } from './client'

// 站点级配置（Logo 等）：随页面加载拉取；未配置 Logo 时返回空字符串（使用内置样式）。
export function useLogoURL(): string {
  const [url, setUrl] = useState('')
  useEffect(() => {
    let alive = true
    void api<{ logo_url?: string }>('/api/config')
      .then((d) => {
        if (alive) setUrl(d.logo_url || '')
      })
      .catch(() => undefined)
    return () => {
      alive = false
    }
  }, [])
  return url
}
