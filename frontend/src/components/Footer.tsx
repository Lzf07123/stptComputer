import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'

interface FooterInfo {
  owner: string
  license: string
  icp: string
  icp_icon: string
  police: string
  police_icon: string
}

const FALLBACK: FooterInfo = {
  owner: '计算机协会 · 汕头职业技术学院',
  license: '开源协议：MIT',
  icp: '',
  icp_icon: '',
  police: '',
  police_icon: '',
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
          {info.license && <span>{info.license}</span>}
          {info.icp ? (
            <a className="beian" href="https://beian.miit.gov.cn/" target="_blank" rel="noreferrer">
              {info.icp_icon && <img src={info.icp_icon} alt="" />}
              {info.icp}
            </a>
          ) : (
            <span className="footer-icp">ICP备案号：待填写</span>
          )}
          {info.police && (
            <a className="beian" href="https://beian.mps.gov.cn/" target="_blank" rel="noreferrer">
              {info.police_icon && <img src={info.police_icon} alt="" />}
              {info.police}
            </a>
          )}
        </div>
        <span className="footer-links">
          <Link to="/join">加入群聊</Link>
          <Link to="/screen">排行榜</Link>
        </span>
      </div>
    </footer>
  )
}
