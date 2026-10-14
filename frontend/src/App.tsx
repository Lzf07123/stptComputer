import { Link, Outlet, useLocation, createBrowserRouter } from 'react-router-dom'
import Home from './pages/Home'
import Join from './pages/Join'
import Quiz from './pages/Quiz'
import Result from './pages/Result'
import Screen from './pages/Screen'
import Admin from './pages/Admin'
import Footer from './components/Footer'

function Layout() {
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
        <Outlet />
      </main>
      {!fullscreen && <Footer />}
    </div>
  )
}

export const router = createBrowserRouter([
  {
    path: '/',
    element: <Layout />,
    children: [
      { index: true, element: <Home /> },
      { path: 'join', element: <Join /> },
      { path: 'quiz', element: <Quiz /> },
      { path: 'result', element: <Result /> },
      { path: 'screen', element: <Screen /> },
      { path: 'admin', element: <Admin /> },
    ],
  },
])
