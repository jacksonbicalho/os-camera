import { useState, useEffect } from 'react'
import { Link, useParams, useLocation, useNavigate } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import SettingsSection from '../../components/SettingsSection'
import UserForm, { type UserFormData } from '../../components/UserForm'
import { authHeaders, clearToken } from '../../auth'

interface Camera {
  id: string
  name?: string
}

interface User {
  id: number
  username: string
  role: 'admin' | 'viewer'
  cameras: string[]
  created_at: string
}

export default function UserDetailSettingsPage() {
  const { id } = useParams<{ id: string }>()
  const location = useLocation()
  const navigate = useNavigate()
  const startEditing = (location.state as { editing?: boolean } | null)?.editing ?? false

  const [user, setUser] = useState<User | null>(null)
  const [cameras, setCameras] = useState<Camera[]>([])
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState(startEditing)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    Promise.all([
      fetch('/api/users', { headers: authHeaders() }),
      fetch('/api/cameras', { headers: authHeaders() }),
    ]).then(async ([ur, cr]) => {
      if (ur.status === 401 || cr.status === 401) { clearToken(); navigate('/login', { replace: true }); return }
      if (ur.status === 403) { navigate('/', { replace: true }); return }
      const users: User[] = await ur.json()
      const found = users.find(u => String(u.id) === id)
      if (!found) { navigate('/settings/users', { replace: true }); return }
      setUser(found)
      setCameras(await cr.json())
    }).catch(() => {}).finally(() => setLoading(false))
  }, [id, navigate])

  const handleUpdate = async (data: UserFormData) => {
    if (!user) return
    setSaving(true)
    setError(null)
    try {
      const res = await fetch(`/api/users/${user.id}`, {
        method: 'PUT',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      })
      if (!res.ok) { setError((await res.text()).trim() || 'Erro ao atualizar usuário'); return }
      const updated: User[] = await (await fetch('/api/users', { headers: authHeaders() })).json()
      const refreshed = updated.find(u => u.id === user.id)
      if (refreshed) setUser(refreshed)
      setEditing(false)
    } finally {
      setSaving(false)
    }
  }

  return (
    <SettingsLayout>
      <div className="mb-6">
        <nav className="flex items-center gap-1.5 text-xs text-gray-500 mb-4">
          <Link to="/settings/users" className="hover:text-gray-300 transition-colors">Usuários</Link>
          <span>/</span>
          <span className="text-gray-300">{user?.username ?? '...'}</span>
        </nav>
        <div className="flex items-center justify-end border-b border-gray-800 pb-2">
          <Link
            to="/settings/users/new"
            className="mb-1 px-3 py-1.5 text-xs bg-blue-600 hover:bg-blue-500 text-white rounded transition-colors"
          >
            + novo usuário
          </Link>
        </div>
      </div>

      {error && (
        <div className="mb-4 px-3 py-2 bg-red-900/30 border border-red-700/50 rounded text-xs text-red-400">
          {error}
        </div>
      )}

      {loading ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : !user ? null : editing ? (
        <UserForm
          cameras={cameras}
          initial={user}
          onSave={handleUpdate}
          onCancel={() => { setEditing(false); setError(null) }}
          saving={saving}
        />
      ) : (
        <div className="flex flex-col gap-4">
          <div className="flex justify-end">
            <button
              onClick={() => { setEditing(true); setError(null) }}
              className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 border border-gray-700 text-gray-300 hover:text-white rounded transition-colors"
            >
              Editar
            </button>
          </div>
          <SettingsSection
            title="Conta"
            fields={[
              { label: 'Username', value: user.username },
              { label: 'Role', value: user.role },
              {
                label: 'Câmeras',
                value: user.role === 'admin'
                  ? 'todas'
                  : user.cameras.length === 0 ? 'nenhuma' : user.cameras.join(', '),
              },
              { label: 'Criado em', value: new Date(user.created_at).toLocaleString('pt-BR') },
            ]}
          />
        </div>
      )}
    </SettingsLayout>
  )
}
