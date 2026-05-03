import { lazy, Suspense } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { getToken } from './auth'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'

const CameraPage = lazy(() => import('./pages/CameraPage'))

function RequireAuth({ children }: { children: React.ReactNode }) {
  return getToken() ? <>{children}</> : <Navigate to="/login" replace />
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
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
