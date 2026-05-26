# Armazenamento

## Estrutura em disco

```
{storage.path}/
└── {camera_id}/
    └── {YYYY}/{MM}/{DD}/
        ├── {HHmmss}.mp4                 ← chunk de gravação
        └── {YYYYMMDDHHmmss}_motion.jpg  ← snapshot do evento de movimento
```

Cada câmera tem seu próprio diretório. Os chunks são nomeados pelo horário de início.

---

## Retenção

A limpeza automática é executada periodicamente (padrão: a cada 60 min) e processa chunks com base em duas regras independentes, configuradas em **Configurações → Armazenamento**:

| Categoria | Comportamento |
|---|---|
| **Com movimento** | Chunks marcados com movimento mais antigos que N. `0` = nunca expira |
| **Sem movimento** | Chunks **sem** movimento mais antigos que N. `0` = desabilitado |

### Ação ao expirar

Para cada categoria é possível escolher o que acontece quando o chunk expira:

| Ação | Descrição |
|---|---|
| **Apagar** | Remove o MP4 e os snapshots JPEG do evento de movimento do disco |
| **Enviar para drive** | Faz upload para um drive S3 configurado e então apaga o arquivo local |

**Exemplo típico:** sem movimento → 1 dia (apagar); com movimento → 7 dias (enviar para drive).

### Como um chunk é marcado com movimento

O sistema sincroniza os chunks do disco com o banco (`recordings`) e marca automaticamente com `has_motion=1` qualquer chunk cujo intervalo de tempo sobrepõe a janela `[evento − segundos_antes, evento + segundos_depois]`.

Isso garante que o chunk anterior e o seguinte a um evento (configurados em **buffer pré/pós-evento**) sejam preservados mesmo que o evento tenha ocorrido no limite de um chunk.

Ver: [Buffer pré/pós-evento](motion.md#buffer-pré/pós-evento)

---

## Limite de tamanho e intervalo

Configurados em **Configurações → Armazenamento**:

| Campo | Padrão | Descrição |
|---|---|---|
| **Máximo (GB)** | `0` | Limite total de disco em GB; `0` = desabilitado |
| **Alerta (%)** | `90` | Percentual do limite que dispara aviso nos logs |
| **Intervalo cleaner** | `60 min` | Frequência da limpeza automática |

---

## Drives S3

Drives permitem enviar gravações para armazenamento externo S3-compatível (AWS S3, Backblaze B2, MinIO, etc.) ao invés de apagar localmente.

### Adicionar um drive

1. Acesse **Configurações → Armazenamento**
2. Role até a seção **Drives** e clique em **+ Adicionar drive**
3. Preencha os campos:

| Campo | Descrição |
|---|---|
| **Nome** | Nome de exibição (ex: "Backblaze B2 Principal") |
| **Endpoint** | URL do endpoint S3 (deixe vazio para AWS S3) |
| **Bucket** | Nome do bucket |
| **Região** | Região do bucket (ex: `us-east-1`) |
| **Access Key / Secret Key** | Credenciais de acesso |
| **Prefixo** | Prefixo opcional para os objetos (ex: `cameras/`) |

4. Após salvar, configure a **ação ao expirar** de uma categoria para **Enviar para drive** e selecione o drive criado.

### Estrutura dos objetos no S3

```
{prefix}/{camera-slug}/{YYYY}/{MM}/{DD}/{HHmmss}.mp4
```

O `camera-slug` é derivado do nome da câmera: letras minúsculas, acentos removidos, espaços substituídos por `-`.

---

## Tela de armazenamento

Em **Configurações → Armazenamento** você pode visualizar e configurar:
- Diretório raiz das gravações
- Retenção por categoria (com/sem movimento) e ação ao expirar
- Intervalo de limpeza e limite de tamanho
- Drives S3 configurados

---

## Sem banco de dados

Se o banco SQLite não estiver disponível na inicialização, o storage consulta o arquivo `motion.ndjson` diretamente para identificar gravações com movimento. O funcionamento é degradado mas a limpeza básica continua operando.

Ver também: [Configuração](configuration.md) | [Detecção de movimento](motion.md)
