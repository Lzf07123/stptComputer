import { Link, Route, Routes, useLocation } from 'react-router-dom'
import Home from './pages/Home'
import Join from './pages/Join'
import Quiz from './pages/Quiz'
import Result from './pages/Result'
import Screen from './pages/Screen'
import Admin from './pages/Admin'
import Footer from './components/Footer'

export default function App() {
  const { pathname } = useLocation()
  const fullscreen = pathname === '/screen'

  const isHome = pathname === '/'

  return (
    <div className={`app${fullscreen ? ' fullscreen' : ''}${isHome ? ' route-home' : ''}`}>
      {!fullscreen && (
        <header className="topbar">
          <Link className="logo" to="/">计算机协会</Link>
          <nav>
            <Link to="/join">加群</Link>
            <Link to="/quiz">答题</Link>
            <Link to="/screen">排行榜</Link>
          </nav>
        </header>
      )}
      <main className="page">
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/join" element={<Join />} />
          <Route path="/quiz" element={<Quiz />} />
          <Route path="/result" element={<Result />} />
          <Route path="/screen" element={<Screen />} />
          <Route path="/admin" element={<Admin />} />
        </Routes>
      </main>
      {!fullscreen && <Footer />}
    </div>
  )
}
