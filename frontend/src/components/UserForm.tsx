import { useState } from 'react'
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
}

export interface UserFormData {
  username: string
  password: string
  role: 'admin' | 'viewer'
  cameras: string[]
}

interface UserFormProps {
  cameras: Camera[]
  initial?: User
  onSave: (data: UserFormData) => Promise<void>
  onCancel: () => void
  saving: boolean
  // Na edição, a senha não é um campo do form — é um fluxo dedicado (ChangePasswordPage).
  onChangePassword?: () => void
}

export default function UserForm({ cameras, initial, onSave, onCancel, saving, onChangePassword }: UserFormProps) {
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
        {!initial && (
          <div>
            <label className="block text-xs text-gray-400 mb-1">Senha</label>
            <input
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              required
              className="w-full bg-gray-950 border border-gray-700 rounded px-3 py-1.5 text-sm text-gray-200 focus:outline-none focus:border-blue-500"
            />
          </div>
        )}
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
                {cam.name || cam.id}
              </button>
            ))}
          </div>
        </div>
      )}

      <div className="flex gap-2 pt-1">
        <Button id="user-form-save" type="submit" size="sm" disabled={saving}>
          {saving ? 'Salvando...' : 'Salvar'}
        </Button>
        <Button id="user-form-cancel" type="button" size="sm" variant="outline" onClick={onCancel}>
          Cancelar
        </Button>
        {initial && onChangePassword && (
          <Button id="user-form-change-password" type="button" size="sm" variant="outline" className="ml-auto" onClick={onChangePassword}>
            Alterar senha
          </Button>
        )}
      </div>
    </form>
  )
}
