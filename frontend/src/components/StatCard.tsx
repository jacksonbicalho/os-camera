import type { ReactNode } from 'react'

interface StatCardProps {
  label: string
  value: ReactNode
  subtext: ReactNode
  colSpan2?: boolean
}

export default function StatCard({ label, value, subtext, colSpan2 = false }: StatCardProps) {
  return (
    <div className={`bg-gray-900 border border-gray-800 rounded-lg p-5${colSpan2 ? ' sm:col-span-2' : ''}`}>
      <p className="text-xs text-gray-400 uppercase tracking-wider font-medium mb-2">{label}</p>
      <p className="text-3xl font-bold text-gray-200">{value}</p>
      <p className="text-xs text-gray-500 mt-1">{subtext}</p>
    </div>
  )
}
