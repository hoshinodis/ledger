import { useEffect, useState } from 'react'
import { nav } from './nav'
import ProjectList from './pages/ProjectList'
import ProjectDetail from './pages/ProjectDetail'
import IssueDetail from './pages/IssueDetail'

type Route =
  | { page: 'projects' }
  | { page: 'project'; pid: number }
  | { page: 'issue'; iid: number }

/** parseRoute maps the supported URL paths to pages. */
function parseRoute(pathname: string): Route {
  const parts = pathname.split('/').filter(Boolean)
  if (parts[0] === 'p' && parts[1] && parts[2] === 'i' && parts[3]) {
    return { page: 'issue', iid: Number(parts[3]) }
  }
  if (parts[0] === 'p' && parts[1]) {
    return { page: 'project', pid: Number(parts[1]) }
  }
  return { page: 'projects' }
}

export default function App() {
  const [path, setPath] = useState(window.location.pathname)

  useEffect(() => {
    const onPop = () => setPath(window.location.pathname)
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  const route = parseRoute(path)
  if (route.page === 'project') {
    return <ProjectDetail id={route.pid} />
  }
  if (route.page === 'issue') {
    return <IssueDetail id={route.iid} />
  }
  return <ProjectList />
}

// Re-exported so page components can navigate consistently.
export { nav }
