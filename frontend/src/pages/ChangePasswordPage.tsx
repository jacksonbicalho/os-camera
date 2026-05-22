import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { getUsername, changePassword, login, clearToken, getRole, authHeaders, mustChangePassword } from '../auth'
import AppLayout from '../components/AppLayout'

export default function ChangePasswordPage() {
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()
  const username = getUsername() ?? ''

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (password !== confirm) {
      setError('As senhas não coincidem')
      return
    }
    if (password.length < 8) {
      setError('A senha deve ter pelo menos 8 caracteres')
      return
    }
    setError('')
    setLoading(true)
    try {
      await changePassword(password)
      clearToken()
      await login(username, password)
      try {
        const res = await fetch('/api/cameras', { headers: authHeaders() })
        const data = res.ok ? await res.json() : []
        if (Array.isArray(data) && data.length === 0 && getRole() === 'admin') {
          navigate('/settings/cameras/new', { replace: true })
          return
        }
      } catch { /* ignore, segue para o dashboard */ }
      navigate('/', { replace: true })
    } catch {
      setError('Falha ao alterar senha. Tente novamente.')
    } finally {
      setLoading(false)
    }
  }

  const form = (
    <div className="w-full max-w-sm bg-gray-900 rounded-lg p-8 shadow-xl border border-gray-800">
      {mustChangePassword() && (
        <p className="text-sm text-gray-400 text-center mb-6">
          Por segurança, defina uma nova senha antes de continuar.
        </p>
      )}
      <h2 className="text-lg font-semibold text-white mb-6">Alterar senha</h2>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div>
          <label className="block text-sm text-gray-400 mb-1">Nova senha</label>
          <input
            type="password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            required
            autoFocus
            minLength={8}
            className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-white text-sm focus:outline-none focus:border-blue-500"
          />
        </div>
        <div>
          <label className="block text-sm text-gray-400 mb-1">Confirmar senha</label>
          <input
            type="password"
            value={confirm}
            onChange={e => setConfirm(e.target.value)}
            required
            className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-white text-sm focus:outline-none focus:border-blue-500"
          />
        </div>
        {error && <p className="text-red-400 text-sm">{error}</p>}
        <button
          type="submit"
          disabled={loading}
          className="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white rounded px-4 py-2 text-sm font-medium transition-colors"
        >
          {loading ? 'Salvando...' : 'Definir nova senha'}
        </button>
      </form>
    </div>
  )

  if (mustChangePassword()) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-950">
        {form}
      </div>
    )
  }

  return (
    <AppLayout>
      <div className="flex justify-center pt-12">
        {form}
      </div>
    </AppLayout>
  )
}
