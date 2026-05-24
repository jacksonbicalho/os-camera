# Usuários

## Papéis

| Papel | Acesso |
|---|---|
| **admin** | Acesso completo: gerencia câmeras, usuários, configurações de servidor e armazenamento |
| **viewer** | Acesso somente leitura: visualiza câmeras, gravações e eventos; não pode alterar configurações |

---

## Primeiro login

O usuário administrador inicial é criado automaticamente na primeira execução com as credenciais definidas em `camera.yaml`:

```yaml
admin:
  username: admin
  password: changeme
```

No primeiro login, o sistema exige troca de senha obrigatória antes de liberar o acesso. A senha do `camera.yaml` não precisa ser atualizada — ela serve apenas para a criação inicial.

---

## Gerenciar usuários

Disponível apenas para administradores em **Configurações → Usuários**.

### Adicionar usuário

1. Clique em **+ Novo usuário**
2. Informe nome de usuário, senha e papel (admin / viewer)
3. Clique em **Salvar**

### Editar usuário

1. Clique no nome do usuário
2. Altere os campos desejados
3. Clique em **Salvar**

### Remover usuário

1. Clique em **Remover** ao lado do usuário
2. Confirme a remoção

> Não é possível remover o próprio usuário com o qual você está logado.

---

## Trocar senha

Qualquer usuário pode trocar sua própria senha em **Configurações → Sistema → Alterar senha**.

Administradores também podem redefinir a senha de outros usuários pela tela de edição de usuário.

---

## Autenticação

O sistema usa **JWT HS256** com expiração de 24 horas. O segredo é gerado aleatoriamente a cada boot — tokens não sobrevivem a reinicializações do servidor.

Para usar um segredo fixo (tokens válidos entre reinicializações):

```yaml
server:
  jwt_secret: "meu-segredo-fixo"
```

Ou via variável de ambiente:

```bash
CAMERA_SERVER_JWT_SECRET="meu-segredo-fixo" ./camera --config camera.yaml
```

O token é aceito por:
- Header HTTP: `Authorization: Bearer <token>`
- Query param: `?token=<token>` (necessário para `<video src>` e streams HLS)

Ver também: [Instalação](installation.md) | [Configuração](configuration.md)
