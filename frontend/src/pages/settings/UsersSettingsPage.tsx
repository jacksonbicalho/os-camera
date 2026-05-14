import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import ConfirmDialog from '../../components/ConfirmDialog'
import { authHeaders, clearToken } from '../../auth'

interface Camera {
  id: string
}

interface User {
  id: number
  username: string
  role: 'admin' | 'viewer'
  cameras: string[]
  created_at: string
}

function RoleBadge({ role }: { role: string }) {
  return (
    <span className={`px-2 py-0.5 text-xs rounded-full font-medium ${
      role === 'admin'
        ? 'bg-blue-900/50 text-blue-300 border border-blue-700/50'
        : 'bg-gray-800 text-gray-400 border border-gray-700'
    }`}>
      {role}
    </span>
  )
}

interface FormData {
  username: string
  password: string
  role: 'admin' | 'viewer'
  cameras: string[]
}

interface UserFormProps {
  cameras: Camera[]
  initial?: User
  onSave: (data: FormData) => Promise<void>
  onCancel: () => void
  saving: boolean
}

function UserForm({ cameras, initial, onSave, onCancel, saving }: UserFormProps) {
  const [username, setUsername] = useState(initial?.username ?? '')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<'admin' | 'viewer'>(initial?.role ?? 'viewer')
  const [selectedCameras, setSelectedCameras] = useState<string[]>(initial?.cameras ?? [])

  const toggleCamera = (id: string) => {
    setSelectedCameras(prev =>
      prev.includes(id) ? prev.filter(c => c !== id) : [...prev, id]
    )
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSave({ username, password, role, cameras: selectedCameras })
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-3">
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div>
          <label className="block text-xs text-gray-400 mb-1">Username</label>
          <input
            value={username}
            onChange={e => setUsername(e.target.value)}
            required
            className="w-full bg-gray-950 border border-gray-700 rounded px-3 py-1.5 text-sm text-gray-200 focus:outline-none focus:border-blue-500"
          />
        </div>
        <div>
          <label className="block text-xs text-gray-400 mb-1">
            {initial ? 'Nova senha (deixe em branco para não alterar)' : 'Senha'}
          </label>
          <input
            type="password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            required={!initial}
            className="w-full bg-gray-950 border border-gray-700 rounded px-3 py-1.5 text-sm text-gray-200 focus:outline-none focus:border-blue-500"
          />
        </div>
        <div>
          <label className="block text-xs text-gray-400 mb-1">Role</label>
          <select
            value={role}
            onChange={e => setRole(e.target.value as 'admin' | 'viewer')}
            className="w-full bg-gray-950 border border-gray-700 rounded px-3 py-1.5 text-sm text-gray-200 focus:outline-none focus:border-blue-500"
          >
            <option value="viewer">viewer</option>
            <option value="admin">admin</option>
          </select>
        </div>
      </div>

      {role === 'viewer' && cameras.length > 0 && (
        <div>
          <label className="block text-xs text-gray-400 mb-2">Câmeras com acesso</label>
          <div className="flex flex-wrap gap-2">
            {cameras.map(cam => (
              <button
                key={cam.id}
                type="button"
                onClick={() => toggleCamera(cam.id)}
                className={`px-3 py-1 text-xs rounded border transition-colors ${
                  selectedCameras.includes(cam.id)
                    ? 'bg-blue-700 border-blue-600 text-white'
                    : 'bg-gray-900 border-gray-700 text-gray-400 hover:border-gray-500'
                }`}
              >
                {cam.id}
              </button>
            ))}
          </div>
        </div>
      )}

      <div className="flex gap-2 pt-1">
        <button
          type="submit"
          disabled={saving}
          className="px-4 py-1.5 text-xs bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded transition-colors"
        >
          {saving ? 'Salvando...' : 'Salvar'}
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="px-4 py-1.5 text-xs text-gray-300 hover:text-white border border-gray-600 rounded transition-colors"
        >
          Cancelar
        </button>
      </div>
    </form>
  )
}

export default function UsersSettingsPage() {
  const navigate = useNavigate()
  const [users, setUsers] = useState<User[]>([])
  const [cameras, setCameras] = useState<Camera[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [saving, setSaving] = useState(false)
  const [deleteId, setDeleteId] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)

  const loadUsers = useCallback(async () => {
    const res = await fetch('/api/users', { headers: authHeaders() })
    if (res.status === 401) { clearToken(); navigate('/login', { replace: true }); return }
    if (res.status === 403) { navigate('/', { replace: true }); return }
    setUsers(await res.json())
  }, [navigate])

  useEffect(() => {
    Promise.all([
      fetch('/api/users', { headers: authHeaders() }),
      fetch('/api/cameras', { headers: authHeaders() }),
    ]).then(async ([ur, cr]) => {
      if (ur.status === 401 || cr.status === 401) { clearToken(); navigate('/login', { replace: true }); return }
      if (ur.status === 403) { navigate('/', { replace: true }); return }
      setUsers(await ur.json())
      setCameras(await cr.json())
    }).catch(() => {}).finally(() => setLoading(false))
  }, [navigate])

  const handleCreate = async (data: FormData) => {
    setSaving(true)
    setError(null)
    try {
      const res = await fetch('/api/users', {
        method: 'POST',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      })
      if (!res.ok) {
        setError((await res.text()).trim() || 'Erro ao criar usuário')
        return
      }
      await loadUsers()
      setCreating(false)
    } finally {
      setSaving(false)
    }
  }

  const handleUpdate = async (id: number, data: FormData) => {
    setSaving(true)
    setError(null)
    try {
      const res = await fetch(`/api/users/${id}`, {
        method: 'PUT',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      })
      if (!res.ok) {
        setError((await res.text()).trim() || 'Erro ao atualizar usuário')
        return
      }
      await loadUsers()
      setEditingId(null)
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async () => {
    if (deleteId == null) return
    try {
      await fetch(`/api/users/${deleteId}`, { method: 'DELETE', headers: authHeaders() })
      await loadUsers()
    } finally {
      setDeleteId(null)
    }
  }

  const userToDelete = users.find(u => u.id === deleteId)

  return (
    <SettingsLayout>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-lg font-semibold text-gray-200">Usuários</h2>
        {!creating && (
          <button
            onClick={() => { setCreating(true); setEditingId(null); setError(null) }}
            className="px-3 py-1.5 text-xs bg-blue-600 hover:bg-blue-500 text-white rounded transition-colors"
          >
            + Novo usuário
          </button>
        )}
      </div>

      {error && (
        <div className="mb-4 px-3 py-2 bg-red-900/30 border border-red-700/50 rounded text-xs text-red-400">
          {error}
        </div>
      )}

      {creating && (
        <div className="mb-4 bg-gray-900 border border-gray-700 rounded-lg p-4">
          <p className="text-xs font-medium text-gray-400 mb-3">Novo usuário</p>
          <UserForm
            cameras={cameras}
            onSave={handleCreate}
            onCancel={() => { setCreating(false); setError(null) }}
            saving={saving}
          />
        </div>
      )}

      {loading ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : users.length === 0 ? (
        <p className="text-gray-500 text-sm">Nenhum usuário.</p>
      ) : (
        <div className="flex flex-col gap-2">
          {users.map(user => (
            <div key={user.id} className="bg-gray-900 border border-gray-800 rounded-lg px-4 py-3">
              {editingId === user.id ? (
                <UserForm
                  cameras={cameras}
                  initial={user}
                  onSave={data => handleUpdate(user.id, data)}
                  onCancel={() => { setEditingId(null); setError(null) }}
                  saving={saving}
                />
              ) : (
                <div className="flex items-center justify-between gap-3">
                  <div className="flex items-center gap-3 min-w-0">
                    <span className="text-sm font-mono text-gray-200 truncate">{user.username}</span>
                    <RoleBadge role={user.role} />
                    {user.role === 'viewer' && (
                      <span className="text-xs text-gray-500 truncate">
                        {user.cameras.length === 0 ? 'sem câmeras' : user.cameras.join(', ')}
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-1 shrink-0">
                    <button
                      onClick={() => { setEditingId(user.id); setCreating(false); setError(null) }}
                      className="px-3 py-1 text-xs text-gray-400 hover:text-white border border-gray-700 rounded transition-colors"
                    >
                      Editar
                    </button>
                    <button
                      onClick={() => setDeleteId(user.id)}
                      className="px-3 py-1 text-xs text-red-500 hover:text-red-400 border border-gray-700 rounded transition-colors"
                    >
                      Remover
                    </button>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      <ConfirmDialog
        open={deleteId != null}
        title="Remover usuário"
        message={`Remover "${userToDelete?.username}"? Esta ação não pode ser desfeita.`}
        confirmLabel="Remover"
        danger
        onConfirm={handleDelete}
        onCancel={() => setDeleteId(null)}
      />
    </SettingsLayout>
  )
}
