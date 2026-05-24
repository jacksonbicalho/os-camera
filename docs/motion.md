# Detecção de movimento

A detecção de movimento analisa frames do stream RTSP via ffmpeg (diff de frames em escala de cinza, resolução reduzida) sem depender de bibliotecas externas de visão computacional.

## Ativar detecção

1. Acesse **Configurações → Câmeras → [câmera]**
2. Clique na aba **Movimento**
3. Marque **Ativar detecção de movimento**
4. Ajuste os parâmetros e clique em **Salvar**

---

## Parâmetros

### Sensibilidade

| Campo | Padrão | Descrição |
|---|---|---|
| **Threshold** | `0.02` | Percentual de pixels alterados para considerar movimento (0–1). Valores menores = mais sensível |
| **FPS de análise** | `2` | Frames por segundo analisados. Valores maiores = menor latência de detecção, maior uso de CPU |

**Ajuste típico:**
- Câmera interna (pouca variação): `0.01–0.03`
- Câmera externa (vento, chuva, sombras): `0.05–0.10`

Use o **gráfico de scores em tempo real** na página da câmera para calibrar — o eixo Y usa escala logarítmica para melhor visualização.

### Cooldown

| Campo | Padrão | Descrição |
|---|---|---|
| **Cooldown (segundos)** | `30` | Tempo mínimo entre dois eventos registrados para a mesma câmera |

### Buffer pré/pós-evento

Controla quais chunks de gravação são preservados ao redor de um evento de movimento:

| Campo | Padrão | Descrição |
|---|---|---|
| **Segundos antes do evento** | `10` | Chunks que terminaram até N segundos antes do evento são marcados com movimento |
| **Segundos após o evento** | `10` | Chunks que começaram até N segundos depois do evento são marcados com movimento |

> **Por que isso importa?** O storage limpa gravações sem movimento primeiro. Se o evento cair no fim de um chunk, o chunk seguinte (que contém a continuação) seria apagado sem essa configuração.

Exemplo com chunks de 5 s e buffer de 10/10:
```
chunk  início     fim     resultado
  A    19:26:20  19:26:25  marcado (termina 7 s antes do evento)
  B    19:26:25  19:26:30  marcado (termina 2 s antes do evento)
  C    19:26:30  19:26:35  marcado (contém o evento 19:26:32)
  D    19:26:35  19:26:40  marcado (começa 3 s depois do evento)
  E    19:26:40  19:26:45  descartado (começa 13 s depois, fora do buffer)
```

### Resolução de captura

| Campo | Padrão | Descrição |
|---|---|---|
| **Automático** | ativado | Usa 25% da resolução original do stream |
| **Percentual** | `25%` | Reduz a resolução antes da análise (menor = mais rápido, menos preciso) |

---

## Zonas de exclusão

Exclui regiões da imagem da análise de movimento — útil para ignorar árvores balançando, estradas movimentadas ou fontes de luz piscando.

1. Na aba **Movimento**, clique em **Zonas de exclusão**
2. Clique em **Nova zona** e desenhe o polígono na imagem
3. Salve

As zonas são aplicadas ao frame antes do cálculo do score de diferença.

---

## Eventos e snapshots

Cada evento detectado gera:
- **Registro no banco** (tabela `motion_events`) com timestamp e score
- **Snapshot JPEG** anotado com bounding box e score sobreposto, salvo em `{storage}/{camera_id}/{data}/{timestamp}_motion.jpg`

Na página da câmera, a aba **Eventos** exibe a lista com thumbnails. Clique em um thumbnail para ver a imagem em tamanho completo.

---

## Score em tempo real

O gráfico **MotionScore** na página da câmera exibe o score bruto de cada frame via SSE (`/api/cameras/{id}/motion/scores`). A janela exibe os últimos 30 segundos em escala logarítmica.

Use para:
- Calibrar o threshold sem precisar provocar movimentos repetidamente
- Confirmar que a câmera está sendo monitorada
- Diagnosticar falsos positivos

---

## Notificações

O sino no topo da interface exibe notificações de eventos de movimento em tempo real. As notificações são persistidas no `localStorage` (máximo 100 por câmera).

Ações disponíveis no painel de notificações:
- Marcar como lida / não lida
- Excluir individualmente ou em lote

Ver também: [Câmeras](cameras.md) | [Armazenamento](storage.md)
