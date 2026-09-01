import { Link, Route, Routes } from 'react-router-dom'

import { EmptyState } from './components/ui'
import { IdeaDetailPage } from './pages/IdeaDetailPage'
import { IdeaListPage } from './pages/IdeaListPage'
import { NewIdeaPage } from './pages/NewIdeaPage'

export function App() {
  return (
    <div className="min-h-full">
      <nav className="border-b border-slate-200 bg-white/80 backdrop-blur dark:border-slate-800 dark:bg-slate-900/80">
        <div className="mx-auto flex w-full max-w-7xl items-center px-4 py-3">
          <Link to="/" className="text-sm font-semibold tracking-tight">
            idpad<span className="text-slate-400"> · idea notebook</span>
          </Link>
        </div>
      </nav>

      <main>
        <Routes>
          <Route path="/" element={<IdeaListPage />} />
          <Route path="/ideas/new" element={<NewIdeaPage />} />
          <Route path="/ideas/:id" element={<IdeaDetailPage />} />
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
