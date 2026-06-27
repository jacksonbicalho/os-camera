// Groups the flat device-info key/value map (from
// GET /api/cameras/{id}/device-info) into labelled, ordered sections for
// display, isolating the raw.* dump.

export interface DeviceInfoField {
  key: string
  label: string
  value: string
}

export interface DeviceInfoSection {
  title: string
  fields: DeviceInfoField[]
}

export interface GroupedDeviceInfo {
  sections: DeviceInfoSection[]
  raw: DeviceInfoField[]
}

const IDENTITY_LABELS: Record<string, string> = {
  model: 'Modelo',
  serial: 'Serial',
  vendor: 'Fabricante',
  connection: 'Conexão',
  firmware: 'Firmware',
  hardware: 'Hardware',
  collector: 'Coletor',
}

const TIME_LABELS: Record<string, string> = {
  'ntp.enabled': 'NTP',
  timezone: 'Fuso horário',
}

const STREAM_FIELD_LABELS: Record<string, string> = {
  codec: 'Codec',
  width: 'Largura',
  height: 'Altura',
  fps: 'FPS',
  gop: 'GOP',
  bitrate: 'Bitrate',
  bitrate_control: 'Controle de bitrate',
}

export function groupDeviceInfo(values: Record<string, string>): GroupedDeviceInfo {
  const used = new Set<string>()
  const sections: DeviceInfoSection[] = []

  const pushSection = (title: string, fields: DeviceInfoField[]) => {
    if (fields.length > 0) sections.push({ title, fields })
  }

  const orderedSection = (
    title: string,
    keys: string[],
    labelFor: (key: string) => string,
  ) => {
    const fields: DeviceInfoField[] = []
    for (const key of keys) {
      if (key in values) {
        fields.push({ key, label: labelFor(key), value: values[key] })
        used.add(key)
      }
    }
    pushSection(title, fields)
  }

  orderedSection(
    'Identidade',
    ['model', 'serial', 'vendor', 'connection', 'firmware', 'hardware', 'collector'],
    (k) => IDENTITY_LABELS[k] ?? k,
  )
  orderedSection('Rede', ['mac'], () => 'MAC')
  orderedSection('Tempo', ['ntp.enabled', 'timezone'], (k) => TIME_LABELS[k] ?? k)

  const streamSection = (title: string, prefix: string) => {
    const fields: DeviceInfoField[] = []
    for (const sub of ['codec', 'width', 'height', 'fps', 'gop', 'bitrate', 'bitrate_control']) {
      const key = prefix + sub
      if (key in values) {
        fields.push({ key, label: STREAM_FIELD_LABELS[sub] ?? sub, value: values[key] })
        used.add(key)
      }
    }
    pushSection(title, fields)
  }
  streamSection('Stream principal', 'stream.main.')
  streamSection('Stream secundário', 'stream.sub.')

  const raw: DeviceInfoField[] = []
  const others: DeviceInfoField[] = []
  for (const [k, v] of Object.entries(values)) {
    if (k.startsWith('raw.')) {
      const key = k.slice('raw.'.length)
      raw.push({ key, label: key, value: v })
    } else if (!used.has(k)) {
      others.push({ key: k, label: k, value: v })
    }
  }
  others.sort((a, b) => a.key.localeCompare(b.key))
  pushSection('Outros', others)

  raw.sort((a, b) => a.key.localeCompare(b.key))
  return { sections, raw }
}
