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

A limpeza automática é executada periodicamente (padrão: a cada 60 min) e apaga chunks com base em duas regras independentes:

| Regra | Campo | Comportamento |
|---|---|---|
| **Com movimento** | `retention.with_motion_minutes` | Apaga gravações marcadas com movimento mais antigas que N minutos. `0` = nunca apaga |
| **Sem movimento** | `retention.without_motion_minutes` | Apaga gravações **sem** movimento mais antigas que N minutos. `0` = desabilitado |

**Exemplo típico:**
```yaml
storage:
  retention:
    with_motion_minutes: 10080     # 7 dias
    without_motion_minutes: 1440   # 1 dia
```

Com essa configuração, cenas paradas são apagadas em 1 dia liberando espaço, enquanto gravações com eventos de movimento ficam por 7 dias.

### Como um chunk é marcado com movimento

O sistema sincroniza os chunks do disco com o banco (`recordings`) e marca automaticamente com `has_motion=1` qualquer chunk cujo intervalo de tempo sobrepõe a janela `[evento − segundos_antes, evento + segundos_depois]`.

Isso garante que o chunk anterior e o seguinte a um evento (configurados em **buffer pré/pós-evento**) sejam preservados mesmo que o evento tenha ocorrido no limite de um chunk.

Ver: [Buffer pré/pós-evento](motion.md#buffer-pré/pós-evento)

---

## Limite de tamanho

```yaml
storage:
  max_size_gb: 20       # limite total em GB
  warn_percent: 90      # aviso quando ultrapassar 90% do limite
```

Quando o uso total ultrapassar `warn_percent`% do limite, o sistema emite um aviso nos logs. O `max_size_gb: 0` desabilita o controle por tamanho.

---

## Intervalo de limpeza

```yaml
storage:
  interval_minutes: 60   # executa limpeza a cada 60 minutos
```

---

## Tela de armazenamento

Em **Configurações → Armazenamento** você pode visualizar:
- Espaço em uso por câmera
- Total de gravações no banco
- Configurações de retenção atuais

---

## Sem banco de dados

Se o banco SQLite não estiver disponível na inicialização, o storage consulta o arquivo `motion.ndjson` diretamente para identificar gravações com movimento. O funcionamento é degradado mas a limpeza básica continua operando.

Ver também: [Configuração](configuration.md) | [Detecção de movimento](motion.md)
