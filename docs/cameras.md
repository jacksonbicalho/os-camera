# Câmeras

## Adicionar câmera manualmente

1. Acesse **Configurações → Câmeras**
2. Clique em **+ Nova câmera**
3. Preencha os campos e clique em **Salvar**

A câmera é iniciada imediatamente após salvar — gravação, HLS e detecção de movimento sobem em paralelo.

---

## Descoberta automática

O sistema consegue localizar câmeras na rede local sem que você precise saber o endereço IP de antemão.

1. Acesse **Configurações → Rastrear câmeras**
2. Clique em **Rastrear**
3. Aguarde o scan (~5–10 s)
4. Clique em **Adicionar** na câmera desejada
5. Informe usuário e senha
6. Se a câmera suportar ONVIF, os streams disponíveis (MainStream, SubStream) aparecem para seleção
7. Escolha o stream → formulário de nova câmera abre pré-preenchido

**Métodos de descoberta:**

| Método | Descrição |
|---|---|
| **ONVIF** | WS-Discovery multicast UDP (`239.255.255.250:3702`); retorna modelo, fabricante e URLs RTSP exatas via `GetStreamUri` |
| **Scan** | Varre todos os IPs da subnet /24 procurando porta 554 aberta |

> A descoberta exige `network_mode: host` no Docker (já configurado no `docker-compose.yml`) para que o multicast ONVIF e o scan de porta alcancem a LAN real.

---

## Campos do formulário

### Identificação

| Campo | Descrição |
|---|---|
| **Nome** | Nome exibido na interface (ex: Sala, Garagem, Entrada) |
| **RTSP URL** | URL completa do stream (`rtsp://usuario:senha@ip:porta/path`) |

### Gravação

| Campo | Padrão | Descrição |
|---|---|---|
| **Gravar em disco** | ativado | Desabilitar mantém HLS e motion funcionando sem gravar MP4 |
| **Duração do chunk** | `5m` | Tamanho de cada arquivo MP4 (ex: `30s`, `5m`, `1h`) |
| **Modo de gravação** | `auto` | `auto` transcodifica HEVC→H.264 automaticamente; `copy` preserva codec original sem custo de CPU; `h264` força H.264 sempre |

### Streaming HLS

| Campo | Padrão | Descrição |
|---|---|---|
| **Modo de vídeo HLS** | `auto` | Igual ao modo de gravação, aplicado ao stream ao vivo |
| **Duração do segmento HLS** | `2 s` | Menor = menor latência; maior = menos overhead |
| **Tamanho da janela HLS** | `5 seg.` | Número de segmentos mantidos na playlist ao vivo |

> **Menor latência:** segmento de 1 s + janela de 2 segmentos ≈ 2–3 s de atraso.

### Vídeo

| Campo | Padrão | Descrição |
|---|---|---|
| **Codec de vídeo** | auto (ffprobe detecta) | Forçar codec se o ffprobe errar na detecção |
| **Áudio** | auto | `Sim` / `Não` / `Auto` |
| **Resolução** | auto | Forçar resolução de captura; auto usa a resolução nativa do stream |

### Reconexão

| Campo | Padrão | Descrição |
|---|---|---|
| **Intervalo de reconexão** | `30s` | Tempo de espera antes de reconectar após queda do stream |

---

## URLs RTSP por fabricante

Formatos comuns de URL RTSP:

| Fabricante | URL típica |
|---|---|
| Dahua | `rtsp://user:pass@ip:554/cam/realmonitor?channel=1&subtype=0` |
| Hikvision | `rtsp://user:pass@ip:554/h264/ch1/main/av_stream` |
| Reolink | `rtsp://user:pass@ip:554/h264Preview_01_main` |
| TP-Link Tapo | `rtsp://user:pass@ip:554/stream1` |
| Genérico | `rtsp://user:pass@ip:554/` |

**Parâmetros Dahua:**
- `channel=N` — canal da câmera (em DVRs/NVRs)
- `subtype=0` — stream principal (alta qualidade)
- `subtype=1` — sub-stream (baixa resolução, menos banda)

> Use **Rastrear câmeras** com ONVIF para obter as URLs exatas automaticamente.

---

## Reordenar câmeras

Na tela de listagem de câmeras, arraste o ícone ⠿ à esquerda de cada câmera para reordenar. A ordem é salva imediatamente.

---

## Editar ou remover câmera

Passe o mouse sobre uma câmera na lista para exibir os ícones de ação:

- **Lápis** — abre o formulário de edição da câmera
- **Lixeira** — remove a câmera; na confirmação é possível marcar **Apagar também as gravações do disco** para remover os arquivos MP4

---

## Badges de status

| Badge | Significado |
|---|---|
| `motion` | Detecção de movimento ativa |
| `rec off` | Gravação em disco desabilitada |

Ver também: [Detecção de movimento](motion.md) | [Armazenamento](storage.md)
