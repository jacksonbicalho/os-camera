# História: Gravação de Câmeras RTSP em Chunks

## História

Como morador que quer monitorar minha residência,
quero que o sistema grave continuamente o stream RTSP de cada câmera em arquivos MP4 segmentados,
para que eu possa recuperar qualquer momento gravado organizado por câmera e data.

## Critérios de aceite

- [ ] O sistema aceita um arquivo de configuração YAML via `--config <path>` ou usa `camera.yaml` como padrão
- [ ] Configurações globais podem ser sobrescritas por variáveis de ambiente
- [ ] Câmeras são sempre definidas no arquivo de configuração
- [ ] Cada câmera tem seu próprio `id`, `rtsp_url`, e pode sobrescrever os defaults globais de `chunk_duration` e `reconnect_interval`
- [ ] Os chunks são gravados em `<storage_path>/<camera_id>/YYYY/MM/DD/YYYYMMDDHHMMSS.mp4` com timestamp UTC
- [ ] O servidor opera em UTC; a conversão de timezone é responsabilidade do cliente
- [ ] Em caso de queda do stream, o sistema reconecta automaticamente no intervalo configurado
- [ ] Ao receber SIGINT ou SIGTERM, o chunk em aberto é finalizado corretamente antes de encerrar
- [ ] Múltiplas câmeras gravam simultaneamente e de forma independente

## Configuração

```yaml
storage:
  path: /data/recordings       # env: STORAGE_PATH

defaults:
  chunk_duration: 5m           # env: CHUNK_DURATION
  reconnect_interval: 10s      # env: RECONNECT_INTERVAL

cameras:
  - id: entrada
    rtsp_url: rtsp://192.168.1.10:554/stream
  - id: quintal
    rtsp_url: rtsp://192.168.1.11:554/stream
    chunk_duration: 10m        # sobrescreve o default
```

## Exemplo de estrutura de arquivos gerada

```
/data/recordings/
  entrada/
    2026/04/30/
      20260430143022.mp4
      20260430143522.mp4
  quintal/
    2026/04/30/
      20260430143015.mp4
```

## Fora de escopo (v1)

- Retenção automática (limpeza de chunks antigos)
- Servidor de streaming ao vivo
- Detecção de movimento
- Interface web ou app
