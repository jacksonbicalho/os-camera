import { useState } from 'react'

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
}

export default function UserForm({ cameras, initial, onSave, onCancel, saving }: UserFormProps) {
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
                {cam.name || cam.id}
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
