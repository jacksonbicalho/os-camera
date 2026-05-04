import { lazy, Suspense } from 'react'
import { Routes, Route, Navigate, useLocation } from 'react-router-dom'
import { getToken } from './auth'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'

const CameraPage = lazy(() => import('./pages/CameraPage'))
const StatsPage = lazy(() => import('./pages/StatsPage'))

function RequireAuth({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  return getToken()
    ? <>{children}</>
    : <Navigate to="/login" state={{ from: location.pathname + location.search }} replace />
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        path="/"
        element={
          <RequireAuth>
            <DashboardPage />
          </RequireAuth>
        }
      />
      <Route
        path="/cameras/:id"
        element={
          <RequireAuth>
            <Suspense>
              <CameraPage />
            </Suspense>
          </RequireAuth>
        }
      />
      <Route
        path="/stats"
        element={
          <RequireAuth>
            <Suspense>
              <StatsPage />
            </Suspense>
          </RequireAuth>
        }
      />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
