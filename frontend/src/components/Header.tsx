import { useState, useRef, useEffect } from 'react'
import { useNavigate, NavLink } from 'react-router-dom'
import { clearToken } from '../auth'

interface HeaderProps {
  username?: string
}

export default function Header({ username = 'usuário' }: HeaderProps) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const navigate = useNavigate()

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  function logout() {
    clearToken()
    navigate('/login')
  }

  return (
    <header className="flex items-center justify-between px-6 py-3 bg-gray-900 border-b border-gray-800">
      <div className="flex items-center gap-6">
        <span className="text-white font-semibold tracking-wide">📷 Camera</span>
        <nav className="flex items-center gap-4">
          <NavLink
            to="/"
            end
            className={({ isActive }) =>
              `text-sm transition-colors ${isActive ? 'text-white' : 'text-gray-400 hover:text-white'}`
            }
          >
            Câmeras
          </NavLink>
          <NavLink
            to="/stats"
            className={({ isActive }) =>
              `text-sm transition-colors ${isActive ? 'text-white' : 'text-gray-400 hover:text-white'}`
            }
          >
            Estatísticas
          </NavLink>
        </nav>
      </div>
      <div className="relative" ref={ref}>
        <button
          onClick={() => setOpen(v => !v)}
          className="flex items-center gap-2 text-sm text-gray-300 hover:text-white transition-colors"
        >
          {username}
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        {open && (
          <div className="absolute right-0 mt-2 w-36 bg-gray-800 border border-gray-700 rounded shadow-lg z-10">
            <button
              onClick={logout}
              className="block w-full text-left px-4 py-2 text-sm text-gray-300 hover:bg-gray-700 hover:text-white"
            >
              Sair
            </button>
          </div>
        )}
      </div>
    </header>
  )
}
