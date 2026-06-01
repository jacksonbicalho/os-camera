# fix(analysis): seletor de modelo mostra custom+base dinâmico após treino

## Contexto

Após treinar um modelo com `yolo12n` (ou qualquer base diferente de `yolov8n`), o seletor
de modelo na página de análise oferece hardcoded `custom+yolov8n` — ignorando qual base foi
usada no treino. O usuário que treinou com `yolo12n` precisa do custom com esse base, mas a
UI não oferece essa opção.

## Solução

Derivar o base model dinamicamente a partir do `cfg.model` atual: extrair a parte não-custom
(strip `custom+`) e montar a opção `custom+<base>` no seletor. Se o modelo atual já for
`custom` (sem base explícita), usar `yolov8n` como fallback.

Nenhuma mudança no backend necessária — a informação já está em `cfg.model`.

## Critérios de Aceitação

- [ ] Seletor com `has_custom_model=true` e modelo base `yolo12n` exibe `custom+yolo12n`
- [ ] Seletor com `has_custom_model=true` e modelo base `yolov8n` exibe `custom+yolov8n`
- [ ] Seletor com `has_custom_model=true` e modelo `custom` exibe `custom+yolov8n` (fallback)
- [ ] Frontend build sem erros

## Revisão

- [x] Aprovado
