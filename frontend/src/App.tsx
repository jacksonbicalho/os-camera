import { lazy, Suspense, useEffect } from 'react'
import { Routes, Route, Navigate, useLocation, useNavigate } from 'react-router-dom'
import { getToken, mustChangePassword } from './auth'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import ChangePasswordPage from './pages/ChangePasswordPage'
import { SidebarItemsProvider } from './contexts/SidebarContext'
import { DisplayModeProvider } from './contexts/DisplayModeContext'
import { AlertProvider } from './contexts/AlertContext'

const CameraPage = lazy(() => import('./pages/CameraPage'))
const StatsPage = lazy(() => import('./pages/StatsPage'))
const NotificationsPage = lazy(() => import('./pages/NotificationsPage'))
const CamerasSettingsPage = lazy(() => import('./pages/settings/CamerasSettingsPage'))
const CameraDetailSettingsPage = lazy(() => import('./pages/settings/CameraDetailSettingsPage'))
const CameraMotionSettingsPage = lazy(() => import('./pages/settings/CameraMotionSettingsPage'))
const CameraZonesSettingsPage = lazy(() => import('./pages/settings/CameraZonesSettingsPage'))
const CameraStatesSettingsPage = lazy(() => import('./pages/settings/CameraStatesSettingsPage'))
const ServerSettingsPage = lazy(() => import('./pages/settings/ServerSettingsPage'))
const StorageSettingsPage = lazy(() => import('./pages/settings/StorageSettingsPage'))
const SystemSettingsPage = lazy(() => import('./pages/settings/SystemSettingsPage'))
const AboutPage = lazy(() => import('./pages/settings/AboutPage'))
const UsersSettingsPage = lazy(() => import('./pages/settings/UsersSettingsPage'))
const UserDetailSettingsPage = lazy(() => import('./pages/settings/UserDetailSettingsPage'))
const DiscoverPage = lazy(() => import('./pages/settings/DiscoverPage'))
const AnalysisSettingsPage = lazy(() => import('./pages/settings/AnalysisSettingsPage'))
const CameraAnalysisSettingsPage = lazy(() => import('./pages/settings/CameraAnalysisSettingsPage'))
const AppearanceSettingsPage = lazy(() => import('./pages/settings/AppearanceSettingsPage'))

function UnauthorizedHandler() {
  const navigate = useNavigate()
  const location = useLocation()
  useEffect(() => {
    const handler = () => navigate('/login', { state: { from: location.pathname + location.search }, replace: true })
    window.addEventListener('camera:unauthorized', handler)
    return () => window.removeEventListener('camera:unauthorized', handler)
  }, [navigate, location])
  return null
}

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
    <DisplayModeProvider>
    <AlertProvider>
    <SidebarItemsProvider>
    <UnauthorizedHandler />
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/change-password" element={<ChangePasswordPage />} />
      <Route path="/" element={<RequireAuth><DashboardPage /></RequireAuth>} />
      <Route path="/cameras/:id" element={<Lazy><CameraPage /></Lazy>} />
      <Route path="/camera/live/:id" element={<Lazy><CameraPage /></Lazy>} />
      <Route path="/camera/recording/:id/:recording_id" element={<Lazy><CameraPage /></Lazy>} />
      <Route path="/stats" element={<Lazy><StatsPage /></Lazy>} />
      <Route path="/notifications" element={<Lazy><NotificationsPage /></Lazy>} />
      <Route path="/settings/cameras" element={<Lazy><CamerasSettingsPage /></Lazy>} />
      <Route path="/settings/cameras/new" element={<Lazy><CamerasSettingsPage /></Lazy>} />
      <Route path="/settings/cameras/edit/:id" element={<Lazy><CameraDetailSettingsPage /></Lazy>} />
      <Route path="/settings/cameras/motion/:id" element={<Lazy><CameraMotionSettingsPage /></Lazy>} />
      <Route path="/settings/cameras/zones/:id" element={<Lazy><CameraZonesSettingsPage /></Lazy>} />
      <Route path="/settings/cameras/analysis/:id" element={<Lazy><CameraAnalysisSettingsPage /></Lazy>} />
      <Route path="/settings/cameras/states/:id" element={<Lazy><CameraStatesSettingsPage /></Lazy>} />
      <Route path="/settings/cameras/:id" element={<Lazy><CameraDetailSettingsPage /></Lazy>} />
      <Route path="/settings/server" element={<Lazy><ServerSettingsPage /></Lazy>} />
      <Route path="/settings/storage" element={<Lazy><StorageSettingsPage /></Lazy>} />
      <Route path="/settings/system" element={<Lazy><SystemSettingsPage /></Lazy>} />
      <Route path="/settings/about" element={<Lazy><AboutPage /></Lazy>} />
      <Route path="/settings/users" element={<Lazy><UsersSettingsPage /></Lazy>} />
      <Route path="/settings/users/new" element={<Lazy><UsersSettingsPage /></Lazy>} />
      <Route path="/settings/users/:id" element={<Lazy><UserDetailSettingsPage /></Lazy>} />
      <Route path="/settings/discover" element={<Lazy><DiscoverPage /></Lazy>} />
      <Route path="/settings/analysis" element={<Lazy><AnalysisSettingsPage /></Lazy>} />
      <Route path="/settings/appearance" element={<Lazy><AppearanceSettingsPage /></Lazy>} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
    </SidebarItemsProvider>
    </AlertProvider>
    </DisplayModeProvider>
  )
}
