# Transmissão ao vivo

Cada câmera habilitada tem uma transmissão ao vivo de baixa latência, exibida na
página da câmera. O servidor gera continuamente um stream **HLS** a partir do
RTSP da câmera; o navegador reproduz esse stream com `hls.js`.

---

## Como funciona

Para cada câmera, um processo `ffmpeg` lê o RTSP e escreve segmentos HLS em
`{segments_path}/{camera_id}/`:

- **Playlist** `index.m3u8` — lista os segmentos mais recentes.
- **Segmentos** `NNNNNN.ts` — trechos de vídeo (padrão: 2 s cada).

O player baixa a playlist, busca os últimos segmentos e os reproduz na borda ao
vivo. Quando o codec do stream não é compatível com o navegador, o servidor
transcodifica para H.264 (configurável por câmera em `hls_video_mode`).

A playlist é servida com `Cache-Control: no-cache` — assim o navegador sempre
revalida e nunca fica preso replayando uma playlist antiga (por exemplo, depois
de a câmera ficar um tempo offline).

### Modo DVR

Com `hls_dvr_seconds > 0`, a janela HLS mantém vários minutos/horas de segmentos
e adiciona `EXT-X-PROGRAM-DATE-TIME`, permitindo retroceder na própria régua ao
vivo. Os parâmetros `hls_dvr_seconds`, `hls_segment_seconds` e `hls_list_size`
são configurados por câmera na UI.

---

## Controles do player

Abaixo do vídeo há uma barra de controles:

- **Mudo** — sincronizado com os controles nativos do vídeo.
- **Velocidade** — 1× a 32× (o limite real do navegador é detectado de forma
  progressiva, com aviso quando excedido).
- **Reprodução contínua** — ao terminar um clipe, avança automaticamente para a
  próxima gravação ou próximo evento.
- **Tela cheia** e **zoom digital** (arrastar para deslocar a imagem ampliada).

Quando a câmera está sem sinal, a live não fica "fingindo" transmissão antiga: o
processo de streaming tenta reconectar automaticamente assim que a câmera volta.

---

## Reprodução de gravações e atalhos de teclado

Ao clicar em **Gravações** (ou em um **Evento**), a página sai do modo ao vivo e
reproduz o trecho correspondente. A régua vertical à direita mostra a posição na
linha do tempo.

Durante a reprodução de uma gravação, com o foco fora de campos de texto:

| Atalho | Ação |
|---|---|
| `Ctrl` + `↑` / `↓` | Navega entre gravações (chunk anterior / próximo) |
| `Ctrl` + `Shift` + `↑` / `↓` | Avança / retrocede **1 segundo** |
| `Ctrl` + `←` / `→` | Avança / retrocede **um frame** (vídeo pausado) |

O passo frame a frame usa a duração real do frame, estimada via
`requestVideoFrameCallback` (com fallback de 1/30 s). A precisão visível é
limitada pela taxa de quadros da gravação — passos menores que `1/fps` caem no
mesmo frame.

> Os atalhos só agem durante a reprodução de gravação (não no modo ao vivo) e são
> ignorados enquanto se digita em um campo de texto.
