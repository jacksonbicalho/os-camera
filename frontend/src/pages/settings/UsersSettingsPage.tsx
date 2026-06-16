import { useState, useEffect, useCallback } from 'react'
import { Link, useNavigate, useLocation } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import ConfirmDialog from '../../components/ConfirmDialog'
import UserForm, { type UserFormData } from '../../components/UserForm'
import RoleBadge from '../../components/RoleBadge'
import { authHeaders, onUnauthorized } from '../../auth'
import { Button } from '@/components/ui/button'

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

export default function UsersSettingsPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const isNewRoute = location.pathname === '/settings/users/new'

  const [users, setUsers] = useState<User[]>([])
  const [cameras, setCameras] = useState<Camera[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(isNewRoute)
  const [saving, setSaving] = useState(false)
  const [deleteId, setDeleteId] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)

  const loadUsers = useCallback(async () => {
    const res = await fetch('/api/users', { headers: authHeaders() })
    if (res.status === 401) { onUnauthorized(); return }
    if (res.status === 403) { navigate('/', { replace: true }); return }
    setUsers(await res.json())
  }, [navigate])

  useEffect(() => {
    Promise.all([
      fetch('/api/users', { headers: authHeaders() }),
      fetch('/api/cameras', { headers: authHeaders() }),
    ]).then(async ([ur, cr]) => {
      if (ur.status === 401 || cr.status === 401) { onUnauthorized(); return }
      if (ur.status === 403) { navigate('/', { replace: true }); return }
      setUsers(await ur.json())
      setCameras(await cr.json())
    }).catch(() => {}).finally(() => setLoading(false))
  }, [navigate])

  const handleCreate = async (data: UserFormData) => {
    setSaving(true)
    setError(null)
    try {
      const res = await fetch('/api/users', {
        method: 'POST',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      })
      if (!res.ok) { setError((await res.text()).trim() || 'Erro ao criar usuário'); return }
      if (isNewRoute) { navigate('/settings/users', { replace: true }); return }
      await loadUsers()
      setCreating(false)
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
      <div className="flex items-start justify-between mb-6">
        <div>
          <h3 className="text-h2 font-semibold text-foreground">Usuários</h3>
          <p className="text-sm text-muted-foreground mt-1">Gerencie usuários e permissões de acesso.</p>
        </div>
        {!creating && (
          <Button
            id="user-create"
            size="sm"
            className="shrink-0"
            onClick={() => { setCreating(true); setError(null) }}
          >
            + Novo usuário
          </Button>
        )}
      </div>

      {error && (
        <div className="mb-4 px-3 py-2 bg-red-900/30 border border-red-700/50 rounded text-xs text-red-400">
          {error}
        </div>
      )}

      {creating && (
        <div className="mb-4 bg-surface border border-border rounded-lg p-4">
          <p className="text-xs font-medium text-muted-foreground mb-3">Novo usuário</p>
          <UserForm
            cameras={cameras}
            onSave={handleCreate}
            onCancel={() => {
              if (isNewRoute) { navigate('/settings/users', { replace: true }); return }
              setCreating(false); setError(null)
            }}
            saving={saving}
          />
        </div>
      )}

      {loading ? (
        <p className="text-muted-foreground text-sm">Carregando...</p>
      ) : users.length === 0 ? (
        <p className="text-muted-foreground text-sm">Nenhum usuário.</p>
      ) : (
        <div className="flex flex-col gap-2">
          {users.map(user => (
            <div key={user.id} className="bg-surface border border-border rounded-lg px-4 py-3">
              <div className="flex items-center gap-3 min-w-0">
                <Link
                  to={`/settings/users/${user.id}`}
                  className="text-sm font-mono text-foreground hover:text-blue-400 transition-colors truncate min-w-0"
                >
                  {user.username}
                </Link>
                <RoleBadge role={user.role} />
                {user.role === 'viewer' && (
                  <span className="text-xs text-muted-foreground truncate">
                    {user.cameras.length === 0 ? 'sem câmeras' : user.cameras.join(', ')}
                  </span>
                )}
                <div className="ml-auto flex items-center gap-1 pl-3 shrink-0">
                  <Button asChild variant="outline" size="sm">
                    <Link to={`/settings/users/${user.id}`} state={{ editing: true }}>
                      Editar
                    </Link>
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="text-destructive hover:text-destructive"
                    onClick={() => setDeleteId(user.id)}
                  >
                    Remover
                  </Button>
                </div>
              </div>
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
