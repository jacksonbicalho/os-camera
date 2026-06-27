# Webcam local (v4l2)

O os-camera consegue usar uma **webcam local** (de notebook ou USB) como câmera —
ela aparece no **Rastrear câmeras** e é adicionada como qualquer outra. Tudo é
feito pelo próprio servidor, **sem Docker e sem dependência extra**: a única
exigência de runtime é o `ffmpeg` (já obrigatório).

## Como funciona

Cada câmera vira 3 processos ffmpeg (gravar, ao vivo/HLS, movimento) que abrem a
fonte **RTSP**. Um dispositivo v4l2 (`/dev/video0`) **só pode ser aberto por um
processo por vez** e não fala RTSP. Por isso o os-camera:

1. Hospeda um **servidor RTSP embutido** (loopback `127.0.0.1:8554`) — biblioteca
   `gortsplib`, dentro do próprio binário (pacote `internal/webcam`).
2. Sobe **um** ffmpeg que lê o `/dev/videoN` e publica nele (`rtsp://127.0.0.1:8554/webcamN`, H.264).
3. Os 3 processos do pipeline leem esse RTSP local — como qualquer câmera.

Isso só acontece quando há webcam detectada; instalação sem webcam não sobe nada.

## Pré-requisitos

- **Linux** com a webcam exposta em `/dev/video*` (a detecção lê `/sys/class/video4linux`).
- `ffmpeg` com suporte a `v4l2` (o build padrão tem).
- O servidor (binário/systemd) roda **na máquina que tem a webcam**.

### Docker

**Funciona out-of-the-box.** O `docker-compose.yml` já monta `/dev` no container
e libera os devices `major 81` (video4linux) via `device_cgroup_rules` — então a
webcam funciona no `docker compose up` normal, sem comando/override extra. É
**inócuo quando não há webcam** (montar `/dev` não falha; a regra de cgroup só
afeta dispositivos de vídeo). Não há serviço/container extra — o restream é
interno ao os-camera.

Se você roda com `docker run` à mão (sem o compose), passe o device:
`docker run --device /dev/video0 …` (ou `-v /dev:/dev --device-cgroup-rule 'c 81:* rmw'`).

Se o publisher falhar, o motivo aparece nos logs: `webcam: publisher ffmpeg encerrou … ffmpeg_stderr=…`.

## Usando

1. Suba o os-camera normalmente (na máquina com a webcam; com o device acessível).
2. Vá em **Configurações → Rastrear câmeras**. A webcam aparece com o método
   **Webcam** e o nome do dispositivo.
3. Clique **Adicionar** — a webcam é registrada direto pela URL do restream
   (`rtsp://127.0.0.1:8554/webcam0`), sem etapa de usuário/senha (não há auth local).
4. A partir daí ela grava, transmite ao vivo e detecta movimento como qualquer câmera.

Dica: liste os devices com `v4l2-ctl --list-devices`.

## Limitações / notas

- Detecção e restream são **Linux/v4l2** apenas.
- O encode é `libx264 -preset ultrafast` (transcode; webcams costumam sair MJPEG/YUYV).
  Encoder por hardware (ex.: Raspberry Pi) é uma melhoria futura.
- A webcam é tratada como uma **câmera RTSP comum** apontando para o restream local —
  nenhuma configuração especial no `camera.yaml`.
