# Plano — State Classification (estilo Frigate)

Classificar um **recorte fixo** do frame de uma câmera em **estados** (aberto/fechado,
ligado/desligado, lixeira na rua/não, piscina coberta/descoberta…), em vez de detectar
objetos. Inspirado no [state classification do Frigate](https://docs.frigate.video/configuration/custom_classification/state_classification/).

Reaproveita peças que já existem no os-camera: canvas de zonas (`BboxCanvas`), fine-tune
do **ultralytics** no serviço YOLO, pipeline de **motion** (frames RGB ao vivo) e
**SSE + notificações**. Saída por SSE/notificações (MQTT é ponte opcional). O serviço
YOLO é opcional — sem ele, a feature fica inativa, sem afetar gravação/HLS/motion.

## Histórias

### S1 · `feat(yolo): modo classificação no serviço (treino + inferência)` ✅ (PR #349)

Fundação, backend Python, independente. Adiciona ao serviço YOLO o caminho `classify`
do ultralytics (`yolov8n-cls`).
- Endpoints: `POST /classify` (imagem/crop → `{predictions:[{label,prob}], top}`),
  `POST /classify/train` + status (reusa `GET /finetune/status/{id}`), `GET /classify/models`.
- Dataset = pastas por classe (padrão ultralytics); guard de tamanho (`l`/`x` barrados).
- Testabilidade: `services/yolo/test_main.py` (pytest) stuba torch/ultralytics/cv2 via
  `sys.modules`; gate via `scripts/yolo-check.sh` + `check.sh` + job `yolo` no CI.

### S2 · `feat(server): config de classificador de estado por câmera`

Schema + API, lado Go. Espelha o modelo de `camera_motion_zones`.
- Tabelas: `camera_state_classifiers` (camera_id, name, threshold, trigger
  `motion`/`interval`, interval_seconds, crop x1/y1/x2/y2, enabled) + classes (≥2) +
  estado atual/histórico.
- API admin CRUD + `GET` do estado atual.
- **AC testável:** CRUD round-trip (sqlite `:memory:`, httptest); validação (≥2 classes,
  crop válido).
- **Depende de:** nada (config pura). Pode ir em paralelo com S1.

### S3 · `feat(analysis): agendador de inferência + verificação de estado`

O motor, lado Go. É o núcleo mais testável.
- No `motion.Monitor` (movimento sobre o crop) e/ou ticker por intervalo → crop do frame
  RGB **ao vivo** (já decodificado no motion) → chama `/classify` → aplica **N leituras
  iguais consecutivas** → persiste o estado confirmado e emite evento na transição.
- Cliente do serviço YOLO injetado (fake nos testes).
- **AC testável:** dada uma sequência de respostas, só confirma após N iguais; respeita
  threshold; persiste; emite só na mudança. (Lógica pura, fácil de cobrir.)
- **Depende de:** S1 (endpoint) + S2 (config/storage).

### S4 · `feat(ui): configurar classificador (crop + classes + treino)`

Frontend. Reusa o canvas de zonas e o fluxo de rótulos.
- Tela (nova aba sob a câmera): nome, desenhar o **retângulo de crop** (reusa
  `BboxCanvas`), definir classes, gatilho/threshold; rotular crops-amostra por estado;
  botão "Treinar" → dataset + S1.
- Geração de candidatos: amostrar a região dos snapshots de movimento/recentes (helper
  pequeno).
- **AC testável:** criar classificador, desenhar crop, rotular, disparar treino, ver
  status (helpers puros testados; resto visual).
- **Depende de:** S1 + S2.

### S5 · `feat(ui): estado atual ao vivo + histórico + notificação na transição`

Fecha o ciclo. Reusa SSE + `NotificationContext` + `user_notifications`.
- Estado atual por classificador ao vivo (SSE), histórico, e notificação na mudança de
  estado.
- **AC testável:** estado atualiza ao vivo; transição gera notificação; histórico
  persiste.
- **Depende de:** S3.

### Opcional/futuro · `feat: ponte MQTT (Home Assistant)`

Só se quiser integração HA. Frigate publica via MQTT; nós nascemos por SSE/notificações,
então MQTT é uma ponte separada e opcional.

---

**Grafo de dependência:** S1 ∥ S2 → S3 → S5; S4 depende de S1+S2 (e ganha amostras de S3).
Caminho para demo mais rápida: **S1 → S2 → S3 → S4 → S5**.

## Diferenças vs. Frigate

- **Sem MQTT/HA por padrão:** saída por SSE + notificações + UI; MQTT vira ponte opcional.
- **Modelo:** `yolov8n-cls` no ultralytics (nossa stack), em vez de MobileNetV2 cru.
- **GPU:** Frigate assume CPU; nós já temos o caminho GPU (RTX 3050). `-cls` nano também
  roda em CPU/RPi (inferência leve, ainda mais em modo intervalo).
- **Frame ao vivo vs. gravação:** o `/analyze` atual lê MP4; para *state* o ideal é frame
  ao vivo — e o `motion.Monitor` já decodifica RGB full-res, então o crop sai dali.
