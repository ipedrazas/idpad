import { Link, Route, Routes, useParams } from 'react-router-dom'

import { ThemeToggle } from './components/ThemeToggle'
import { EmptyState } from './components/ui'
import { IdeaDetailPage } from './pages/IdeaDetailPage'
import { IdeaListPage } from './pages/IdeaListPage'
import { NewIdeaPage } from './pages/NewIdeaPage'

/**
 * Mounts a fresh detail page per idea. Following a connection between two
 * ideas keeps the same route, and the detail page — like the editor inside it
 * — seeds its local copy of the document once and then owns it. Keying on the
 * id turns that navigation into a remount, so nothing of the previous idea
 * survives into the next.
 */
function KeyedIdeaDetailPage() {
  const { id } = useParams<{ id: string }>()
  return <IdeaDetailPage key={id} />
}

export function App() {
  return (
    <div className="min-h-full">
      <nav className="border-b border-slate-200 bg-white/80 backdrop-blur dark:border-slate-800 dark:bg-slate-900/80">
        <div className="mx-auto flex w-full max-w-7xl items-center px-4 py-3">
          <Link to="/" className="text-sm font-semibold tracking-tight">
            idpad<span className="text-slate-400"> · idea notebook</span>
          </Link>
          <ThemeToggle className="ml-auto" />
        </div>
      </nav>

      <main>
        <Routes>
          <Route path="/" element={<IdeaListPage />} />
          <Route path="/ideas/new" element={<NewIdeaPage />} />
          <Route path="/ideas/:id" element={<KeyedIdeaDetailPage />} />
          <Route
            path="*"
            element={
              <div className="mx-auto w-full max-w-md px-4 py-20">
                <EmptyState title="Page not found" description="That route does not exist." />
              </div>
            }
          />
        </Routes>
      </main>
    </div>
  )
}
