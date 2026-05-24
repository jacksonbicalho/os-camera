import { lazy, Suspense } from 'react'
import { Routes, Route, Navigate, useLocation } from 'react-router-dom'
import { getToken, mustChangePassword } from './auth'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import ChangePasswordPage from './pages/ChangePasswordPage'
import { SidebarItemsProvider } from './contexts/SidebarContext'

const CameraPage = lazy(() => import('./pages/CameraPage'))
const StatsPage = lazy(() => import('./pages/StatsPage'))
const CamerasSettingsPage = lazy(() => import('./pages/settings/CamerasSettingsPage'))
const CameraDetailSettingsPage = lazy(() => import('./pages/settings/CameraDetailSettingsPage'))
const CameraMotionSettingsPage = lazy(() => import('./pages/settings/CameraMotionSettingsPage'))
const CameraZonesSettingsPage = lazy(() => import('./pages/settings/CameraZonesSettingsPage'))
const ServerSettingsPage = lazy(() => import('./pages/settings/ServerSettingsPage'))
const StorageSettingsPage = lazy(() => import('./pages/settings/StorageSettingsPage'))
const SystemSettingsPage = lazy(() => import('./pages/settings/SystemSettingsPage'))
const AboutPage = lazy(() => import('./pages/settings/AboutPage'))
const UsersSettingsPage = lazy(() => import('./pages/settings/UsersSettingsPage'))
const UserDetailSettingsPage = lazy(() => import('./pages/settings/UserDetailSettingsPage'))

function RequireAuth({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  if (!getToken()) return <Navigate to="/login" state={{ from: location.pathname + location.search }} replace />
  if (mustChangePassword()) return <Navigate to="/change-password" replace />
  return <>{children}</>
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
    <SidebarItemsProvider>
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/change-password" element={<ChangePasswordPage />} />
      <Route path="/" element={<RequireAuth><DashboardPage /></RequireAuth>} />
      <Route path="/cameras/:id" element={<Lazy><CameraPage /></Lazy>} />
      <Route path="/stats" element={<Lazy><StatsPage /></Lazy>} />
      <Route path="/settings/cameras" element={<Lazy><CamerasSettingsPage /></Lazy>} />
      <Route path="/settings/cameras/new" element={<Lazy><CamerasSettingsPage /></Lazy>} />
      <Route path="/settings/cameras/:id" element={<Lazy><CameraDetailSettingsPage /></Lazy>} />
      <Route path="/settings/cameras/:id/motion" element={<Lazy><CameraMotionSettingsPage /></Lazy>} />
      <Route path="/settings/cameras/:id/motion/zones" element={<Lazy><CameraZonesSettingsPage /></Lazy>} />
      <Route path="/settings/server" element={<Lazy><ServerSettingsPage /></Lazy>} />
      <Route path="/settings/storage" element={<Lazy><StorageSettingsPage /></Lazy>} />
      <Route path="/settings/system" element={<Lazy><SystemSettingsPage /></Lazy>} />
      <Route path="/settings/about" element={<Lazy><AboutPage /></Lazy>} />
      <Route path="/settings/users" element={<Lazy><UsersSettingsPage /></Lazy>} />
      <Route path="/settings/users/new" element={<Lazy><UsersSettingsPage /></Lazy>} />
      <Route path="/settings/users/:id" element={<Lazy><UserDetailSettingsPage /></Lazy>} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
    </SidebarItemsProvider>
  )
}
