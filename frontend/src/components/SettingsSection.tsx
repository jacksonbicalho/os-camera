interface FieldRow {
  label: string
  value: React.ReactNode
}

interface SettingsSectionProps {
  title: string
  fields: FieldRow[]
}

export default function SettingsSection({ title, fields }: SettingsSectionProps) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-lg overflow-hidden">
      <p className="text-xs text-gray-400 uppercase tracking-wider font-medium px-5 pt-4 pb-3 border-b border-gray-800">
        {title}
      </p>
      <dl className="divide-y divide-gray-800">
        {fields.map(({ label, value }) => (
          <div key={label} className="flex items-baseline gap-4 px-5 py-3">
            <dt className="text-xs text-gray-500 w-44 shrink-0">{label}</dt>
            <dd className="text-sm text-gray-200 font-mono break-all">{value ?? '—'}</dd>
          </div>
        ))}
      </dl>
    </div>
  )
}
