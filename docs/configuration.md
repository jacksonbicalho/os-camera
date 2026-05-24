# Configuração

O arquivo `camera.yaml` é o **bootstrap mínimo** do sistema. Ele define apenas o necessário para o servidor iniciar pela primeira vez. Toda configuração de câmeras, detecção de movimento e zonas de exclusão é feita via interface web e persistida no banco SQLite.

Use o wizard interativo para gerar o arquivo:

```bash
camera init
camera init --output /etc/camera/camera.yaml
```

Ou copie e edite o exemplo:

```bash
cp camera.yaml.example camera.yaml
```

---

## Referência completa

```yaml
debug: false
timezone: America/Sao_Paulo

db_path: /var/camera/data/camera.db

log:
  output: stdout        # stdout | file
  path:                 # diretório quando output: file

server:
  port: 8080
  segments_path: /var/camera/data/hls
  hls_dvr_seconds: 0
  jwt_secret: ""

storage:
  path: /var/camera/data/recordings
  retention:
    with_motion_minutes: 10080
    without_motion_minutes: 1440
  interval_minutes: 60
  max_size_gb: 20
  warn_percent: 90

admin:
  username: admin
  password: changeme
```

---

## Campos

### Raiz

| Campo | Padrão | Descrição |
|---|---|---|
| `debug` | `false` | Ativa logs de nível debug |
| `timezone` | `UTC` | Fuso horário para logs e nomes de arquivo (ex: `America/Sao_Paulo`) |
| `db_path` | — | Caminho do banco SQLite; criado automaticamente se não existir |

### `log`

| Campo | Padrão | Descrição |
|---|---|---|
| `output` | `stdout` | Destino dos logs: `stdout` ou `file` |
| `path` | — | Diretório dos arquivos de log quando `output: file`; gera `debug.log`, `info.log`, `warn.log`, `error.log` |

### `server`

| Campo | Padrão | Descrição |
|---|---|---|
| `port` | — | Porta HTTP da interface web e API |
| `segments_path` | — | Diretório para os segmentos HLS do streaming ao vivo |
| `hls_dvr_seconds` | `0` | Janela DVR em segundos; `0` desabilita (HLS ao vivo sem seek) |
| `jwt_secret` | `""` | Segredo JWT fixo; vazio = gerado aleatoriamente a cada boot (tokens não sobrevivem a reinicializações) |

### `storage`

| Campo | Padrão | Descrição |
|---|---|---|
| `path` | — | Diretório raiz das gravações |
| `retention.with_motion_minutes` | `0` | Retenção de gravações **com** movimento detectado; `0` = nunca apaga |
| `retention.without_motion_minutes` | `0` | Retenção de gravações **sem** movimento; `0` = desabilitado |
| `interval_minutes` | `60` | Intervalo entre execuções da limpeza automática |
| `max_size_gb` | `0` | Limite total de disco em GB; `0` = desabilitado |
| `warn_percent` | `90` | Percentual do `max_size_gb` que dispara aviso de disco cheio |

### `admin`

| Campo | Descrição |
|---|---|
| `username` | Usuário administrador criado na **primeira** inicialização |
| `password` | Senha inicial; o sistema exige troca obrigatória no primeiro login |

> Esses campos só têm efeito na primeira execução. Após a criação do usuário, a senha é gerenciada pela interface web.

---

## Variáveis de ambiente

As variáveis de ambiente sobrescrevem os campos correspondentes do `camera.yaml`:

| Variável | Campo sobrescrito |
|---|---|
| `CAMERA_TIMEZONE` | `timezone` |
| `CAMERA_SERVER_JWT_SECRET` | `server.jwt_secret` |

---

## Estrutura de diretórios

Após a primeira execução, os dados ficam organizados assim:

```
{storage.path}/
└── {camera_id}/
    └── {YYYY}/{MM}/{DD}/
        ├── {HHmmss}.mp4                 ← chunk de gravação
        └── {YYYYMMDDHHmmss}_motion.jpg  ← snapshot do evento de movimento

{server.segments_path}/
└── {camera_id}/
    ├── index.m3u8     ← playlist HLS ao vivo
    └── *.ts           ← segmentos de vídeo

{db_path}              ← banco SQLite (câmeras, usuários, eventos, gravações)
```

Ver também: [Armazenamento](storage.md)
