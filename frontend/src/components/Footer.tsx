import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'

interface FooterInfo {
  owner: string
  license: string
  icp: string
}

const FALLBACK: FooterInfo = {
  owner: '计算机协会 · 汕头职业技术学院',
  license: '开源协议：MIT',
  icp: '',
}

export default function Footer() {
  const [info, setInfo] = useState<FooterInfo>(FALLBACK)

  useEffect(() => {
    void api<{ footer?: FooterInfo }>('/api/config')
      .then((d) => {
        if (d.footer) setInfo(d.footer)
      })
      .catch(() => undefined)
  }, [])

  return (
    <footer className="footer">
      <div className="footer-inner">
        <div className="footer-meta">
          <span>
            © {new Date().getFullYear()} {info.owner || FALLBACK.owner}
          </span>
          <span>{info.license || FALLBACK.license}</span>
          <span className="footer-icp">
            {info.icp ? `ICP备案号：${info.icp}` : 'ICP备案号：待填写'}
          </span>
        </div>
        <span className="footer-links">
          <Link to="/join">加入群聊</Link>
          <Link to="/screen">排行榜</Link>
        </span>
      </div>
    </footer>
  )
}
