import { Card } from "./ui/card";

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
    <Card className="overflow-hidden">
      <p className="text-h4 text-muted-foreground uppercase tracking-wider font-medium px-5 pt-4 pb-3 border-b border-border">
        {title}
      </p>

      {groups ? (
        <div className="divide-y divide-border">
          {groups.map((group, i) => (
            <div
              key={i}
              className={group.length === 2 ? 'grid grid-cols-2 divide-x divide-border' : group.length === 3 ? 'grid grid-cols-3 divide-x divide-border' : ''}
            >
              {group.map(({ label, value }) => (
                <div key={label} className="px-5 py-3">
                  <dt className="mb-1 text-xs text-muted-foreground">{label}</dt>
                  <dd className="break-all font-mono text-sm text-foreground">{value ?? '—'}</dd>
                </div>
              ))}
            </div>
          ))}
        </div>
      ) : (
        <dl className="divide-y divide-border">
          {(fields ?? []).map(({ label, value }) => (
            <div key={label} className="px-5 py-3">
              <dt className="mb-1 text-xs text-muted-foreground">{label}</dt>
              <dd className="break-all font-mono text-sm text-foreground">{value ?? '—'}</dd>
            </div>
          ))}
        </dl>
      )}

      {children}
    </Card>
  );
}
