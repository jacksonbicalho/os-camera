import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import './index.css'
import App from './App.tsx'
import { NotificationProvider } from './contexts/NotificationContext'
import { UserNotificationProvider } from './contexts/UserNotificationContext'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <NotificationProvider>
        <UserNotificationProvider>
          <App />
        </UserNotificationProvider>
      </NotificationProvider>
    </BrowserRouter>
  </StrictMode>,
)
