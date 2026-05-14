import { lazy, Suspense } from 'react'
import { Routes, Route, Navigate, useLocation } from 'react-router-dom'
import { getToken } from './auth'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'

const CameraPage = lazy(() => import('./pages/CameraPage'))
const StatsPage = lazy(() => import('./pages/StatsPage'))
const StatsSettingsPage = lazy(() => import('./pages/settings/StatsSettingsPage'))
const CamerasSettingsPage = lazy(() => import('./pages/settings/CamerasSettingsPage'))
const CameraDetailSettingsPage = lazy(() => import('./pages/settings/CameraDetailSettingsPage'))
const CameraMotionSettingsPage = lazy(() => import('./pages/settings/CameraMotionSettingsPage'))
const CameraZonesSettingsPage = lazy(() => import('./pages/settings/CameraZonesSettingsPage'))
const ServerSettingsPage = lazy(() => import('./pages/settings/ServerSettingsPage'))
const StorageSettingsPage = lazy(() => import('./pages/settings/StorageSettingsPage'))
const SystemSettingsPage = lazy(() => import('./pages/settings/SystemSettingsPage'))
const AboutPage = lazy(() => import('./pages/settings/AboutPage'))
const UsersSettingsPage = lazy(() => import('./pages/settings/UsersSettingsPage'))

function RequireAuth({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  return getToken()
    ? <>{children}</>
    : <Navigate to="/login" state={{ from: location.pathname + location.search }} replace />
}

function Lazy({ children }: { children: React.ReactNode }) {
  return (
    <RequireAuth>
      <Suspense>{children}</Suspense>
    </RequireAuth>
  )
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<RequireAuth><DashboardPage /></RequireAuth>} />
      <Route path="/cameras/:id" element={<Lazy><CameraPage /></Lazy>} />
      <Route path="/stats" element={<Lazy><StatsPage /></Lazy>} />
      <Route path="/settings/stats" element={<Lazy><StatsSettingsPage /></Lazy>} />
      <Route path="/settings/cameras" element={<Lazy><CamerasSettingsPage /></Lazy>} />
      <Route path="/settings/cameras/:id" element={<Lazy><CameraDetailSettingsPage /></Lazy>} />
      <Route path="/settings/cameras/:id/motion" element={<Lazy><CameraMotionSettingsPage /></Lazy>} />
      <Route path="/settings/cameras/:id/motion/zones" element={<Lazy><CameraZonesSettingsPage /></Lazy>} />
      <Route path="/settings/server" element={<Lazy><ServerSettingsPage /></Lazy>} />
      <Route path="/settings/storage" element={<Lazy><StorageSettingsPage /></Lazy>} />
      <Route path="/settings/system" element={<Lazy><SystemSettingsPage /></Lazy>} />
      <Route path="/settings/about" element={<Lazy><AboutPage /></Lazy>} />
      <Route path="/settings/users" element={<Lazy><UsersSettingsPage /></Lazy>} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
