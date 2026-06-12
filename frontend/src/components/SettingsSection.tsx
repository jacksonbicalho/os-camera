interface FieldRow {
  label: string;
  value: React.ReactNode;
}

interface SettingsSectionProps {
  title: string;
  fields?: FieldRow[];
  groups?: FieldRow[][];
  children?: React.ReactNode;
}

export default function SettingsSection({
  title,
  fields,
  groups,
  children,
}: SettingsSectionProps) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-lg overflow-hidden">
      <p className="text-h4 text-gray-400 uppercase tracking-wider font-medium px-5 pt-4 pb-3 border-b border-gray-800">
        {title}
      </p>

      {groups ? (
        <div className="divide-y divide-gray-800">
          {groups.map((group, i) => (
            <div
              key={i}
              className={group.length === 2 ? 'grid grid-cols-2 divide-x divide-gray-800' : group.length === 3 ? 'grid grid-cols-3 divide-x divide-gray-800' : ''}
            >
              {group.map(({ label, value }) => (
                <div key={label} className="px-5 py-3">
                  <dt className="mb-1 text-xs text-gray-500">{label}</dt>
                  <dd className="break-all font-mono text-sm text-gray-200">{value ?? '—'}</dd>
                </div>
              ))}
            </div>
          ))}
        </div>
      ) : (
        <dl className="divide-y divide-gray-800">
          {(fields ?? []).map(({ label, value }) => (
            <div key={label} className="px-5 py-3">
              <dt className="mb-1 text-xs text-gray-500">{label}</dt>
              <dd className="break-all font-mono text-sm text-gray-200">{value ?? '—'}</dd>
            </div>
          ))}
        </dl>
      )}

      {children}
    </div>
  );
}
