---
title: Desenho proposto do cliente FreePBX GraphQL no AMIgow
status: proposed-not-implemented
last_verified: 2026-09-22
---

# Desenho proposto do cliente FreePBX GraphQL no AMIgow

> Este documento é uma proposta técnica, não uma decisão aceita nem uma
> implementação. O primeiro recurso e a política de escrita precisam ser
> escolhidos antes de alterar o código.

## Objetivo

Adicionar ao AMIgow um adaptador tipado para capacidades administrativas do
FreePBX sem substituir a AMI, expor GraphQL arbitrário pela API HTTP do AMIgow
ou acoplar regras de produto ao schema dos módulos FreePBX.

## Matriz de contrato

| Aspecto | Responsável | Regra |
|---|---|---|
| Contrato GraphQL | FreePBX + módulos instalados | Descoberto por scope e introspecção por PBX |
| Cliente e tradução | AMIgow | API Go tipada por caso de uso; sem passthrough GraphQL |
| Regra de produto/tenant | SistemaSafeHouse | Continua fonte de verdade; o PBX não decide condomínio/morador |
| Runtime telefônico | Asterisk/AMI | Eventos, canais, chamadas e fila ao vivo permanecem na AMI |
| Autenticação | FreePBX OAuth2 | Aplicação M2M exclusiva por instância/ambiente |
| Segredos | Host do AMIgow | Arquivos protegidos ou secret store; nunca Git/config/log |
| Apply Config | Operação governada | Mutation separada, checagem AMI e acompanhamento assíncrono |
| Compatibilidade | AMIgow | Detectar capacidade; falhar fechado quando operação não existir |

## Transporte e endpoints

Por padrão:

```text
POST https://<pbx-fqdn>/admin/api/api/token
POST https://<pbx-fqdn>/admin/api/api/gql
```

Mesmo rodando na mesma VPS, o cliente deve usar HTTPS com validação normal do
certificado. Para manter o tráfego local sem desativar TLS, o FQDN do certificado
pode resolver para `127.0.0.1` no próprio host. Não adicionar
`InsecureSkipVerify` ao código.

O PBX de desenvolvimento negou HTTP local com 403. O uso de
`http://127.0.0.1` não é uma opção portável.

## Autenticação OAuth2

O fluxo indicado para o serviço é `client_credentials`:

1. o operador cria uma aplicação Machine-to-Machine pela GUI;
2. a aplicação recebe apenas os scopes necessários;
3. AMIgow autentica o token endpoint com HTTP Basic (`client_id` e
   `client_secret`);
4. envia `grant_type=client_credentials` e `scope=<lista separada por espaço>`;
5. guarda o bearer token somente em memória;
6. reutiliza o token até `expires_in - refresh_skew`;
7. apenas uma goroutine renova o token quando várias requisições concorrem.

Não usar refresh token para este fluxo. Se a renovação falhar, manter o token
anterior apenas enquanto ele ainda for válido; depois disso, falhar fechado.

### Separação de aplicações

Uma única credencial com todos os scopes transforma qualquer comprometimento do
AMIgow em administração total do PBX. O desenho recomendado separa:

| Aplicação | Scopes típicos | Dados/poder |
|---|---|---|
| Inventário | Framework system/modules + Core extension read | Baixo risco relativo, sem escrita |
| CDR | `gql:cdr:read` | PII e histórico de chamadas |
| Writer de um recurso | Scope write mínimo do recurso | Mudança persistente no PBX |
| Apply Config | Framework system write, se aprovado | Operação de alto impacto e assíncrona |

Se o módulo não oferecer granularidade suficiente, a ampliação deve ser tratada
como risco explícito, não escondida em `gql:<module>`.

## Configuração proposta

O formato abaixo é apenas o contrato desejado. O loader atual do AMIgow ainda
não possui esses campos nem suporte a secret files.

```json
{
  "freepbx_api": {
    "enabled": false,
    "base_url": "https://pbx-interno.example.com",
    "token_path": "/admin/api/api/token",
    "graphql_path": "/admin/api/api/gql",
    "client_id_file": "/etc/amigow/secrets/freepbx-client-id",
    "client_secret_file": "/etc/amigow/secrets/freepbx-client-secret",
    "scopes": [
      "gql:framework:read:system",
      "gql:framework:read:modules",
      "gql:core:read:extension"
    ],
    "request_timeout_seconds": 5,
    "token_refresh_skew_seconds": 60,
    "max_response_bytes": 2097152
  }
}
```

Regras:

- `base_url` não pode conter usuário, senha, query ou fragmento;
- paths devem ser fixos ou validados, sem URL arbitrária por requisição;
- arquivos de segredo devem ser regulares, não world-readable e lidos no
  startup;
- não fazer fallback para valor secreto inline;
- configuração inválida com `enabled: true` deve impedir o recurso de ficar
  pronto, mas não deve derrubar as funções AMI não relacionadas;
- falha GraphQL deve degradar apenas os endpoints que dependem dela.

## Organização de código proposta

O pacote existente `internal/freepbx` consulta metadados locais do FreePBX. Para
não misturar banco, AMI e HTTP, usar um pacote distinto:

```text
internal/freepbxapi/
  client.go          # HTTP, limites, headers e decode
  oauth.go           # token manager concorrente
  errors.go          # erros tipados e classificação
  capabilities.go    # schema/version/capability checks
  operations/
    system.go
    extensions.go
    cdr.go
```

Cada arquivo em `operations/` expõe métodos de domínio, por exemplo
`FetchAsteriskDetails` ou `ListExtensions`. Não expor um método HTTP do tipo
`Execute(query string, variables any)` para handlers externos.

Uma interface mínima para testes pode ser:

```go
type GraphQLDoer interface {
    Do(ctx context.Context, operationName string, query string, variables any, out any) error
}
```

Ela deve permanecer interna. Os handlers usam interfaces tipadas por recurso.

## Envelope GraphQL e erros

Uma resposta GraphQL pode ter HTTP 200 e, ao mesmo tempo, conter `errors` e
`data` parcial. O decoder precisa representar os dois:

```go
type response[T any] struct {
    Data   T              `json:"data"`
    Errors []graphqlError `json:"errors"`
}
```

Classificação mínima:

| Classe | Exemplos | Retry automático |
|---|---|---|
| Configuração/autorização | scope ausente, token negado, operação fora do schema | Não |
| Contrato | campo/argumento inválido, erro de decode | Não |
| Domínio | `status: false`, objeto inexistente, validação FreePBX | Não |
| Transporte transitório | timeout antes de resposta, reset, 502/503 | Apenas query idempotente |
| Limite | 429 com `Retry-After` | Apenas query, com limite rígido |
| Mutation incerta | conexão caiu após envio | Nunca repetir cegamente; reconciliar por leitura |

No baseline, request sem bearer no endpoint GraphQL retornou HTTP 500 com uma
negação OAuth no JSON. O classificador deve reconhecer a falha de autenticação e
evitar um loop de retry, sem depender de essa resposta específica em outras
versões.

## Política de retry

- Queries read-only: no máximo duas novas tentativas, exponential backoff com
  jitter e deadline total do contexto.
- Token: uma nova tentativa apenas para falha de transporte/5xx.
- Mutations: zero retry automático.
- `doreload`: zero retry. Se o resultado inicial ficar incerto, consultar o
  estado e a lista de transações antes de qualquer nova solicitação.
- Nunca transformar erro permanente em indisponibilidade silenciosa.

## Operações de leitura iniciais

### Estado do Asterisk

O primeiro smoke test deve pedir somente os campos necessários:

```graphql
query AMIgowFetchAsteriskDetails {
  fetchAsteriskDetails {
    status
    message
    asteriskStatus
    asteriskVersion
    amiStatus
  }
}
```

Os nomes finais precisam ser confirmados pela introspecção do PBX alvo.

### Ramais

O catálogo oficial usa Relay/paginação:

```graphql
query AMIgowListExtensions($first: Int, $after: String) {
  fetchAllExtensions(first: $first, after: $after) {
    status
    message
    totalCount
    extension {
      extensionId
    }
  }
}
```

O tipo de `after` deve ser copiado do schema real; exemplos públicos antigos não
são suficientes para gerar os tipos Go.

## Mutations e Apply Config

Mutation bem-sucedida e configuração aplicada são estados diferentes.

Para ramais com o módulo local `extension_profiles`, não usar `addExtension`
como se ele aceitasse o select da GUI. O input padrão não expõe o nome do
perfil. A proposta de mutation pertencente ao próprio módulo, incluindo
validação, reconciliação e testes, está em
[extension-profiles.md](extension-profiles.md).

```text
mutation persistida
      |
      v
leitura de reconciliação confirma objeto
      |
      v
fetchNeedReload
      |
      v
janela operacional aprovada + AMI confirma zero chamadas
      |
      v
doreload -> transaction_id
      |
      v
fetchApiStatus até sucesso/falha/timeout
      |
      v
AMI/FreePBX readiness + leitura final
```

Regras propostas:

1. O método que altera um recurso não chama `doreload` implicitamente.
2. O chamador escolhe entre apenas persistir e solicitar apply em uma operação
   separada.
3. Antes do apply, a AMI confirma zero chamadas e ausência de operação de reload
   concorrente.
4. O retorno inicial de `doreload` significa “iniciado”, não “concluído”.
5. `fetchApiStatus` deve chegar a um estado terminal dentro de um timeout.
6. Depois do apply, confirmar Asterisk/AMI e reler o recurso.
7. Bugs conhecidos de módulos podem não marcar `needreload`; a verificação do
   objeto e os testes por versão continuam obrigatórios.

## Idempotência e reconciliação

Nem toda mutation do FreePBX oferece idempotency key. Para cada integração de
escrita, definir antes:

- chave natural do objeto (`extensionId`, DID, group number etc.);
- query para verificar existência e conteúdo;
- comportamento quando o estado já corresponde ao desejado;
- comportamento quando existe com configuração conflitante;
- compensação ou instrução operacional quando o resultado é incerto.

Não implementar “delete then create” genérico. Algumas APIs do FreePBX fazem
isso internamente e podem perder campos que o caller não enviou.

## Segurança da API exposta pelo AMIgow

O AMIgow não deve virar um proxy GraphQL. Cada endpoint público precisa:

- representar um caso de uso explícito;
- validar identificadores e campos permitidos;
- aplicar autenticação própria e autorização de tenant quando houver chamada da
  SistemaSafeHouse;
- não aceitar nome de operation, query, scope, URL ou seleção de campos enviados
  pelo cliente;
- remover segredos e PII dos logs;
- limitar tamanho de request/response e tempo total.

Isso é especialmente importante porque a chave HTTP atual do AMIgow já concede
ações AMI de alto poder. Não ampliar essa chave automaticamente para mutations
administrativas do FreePBX.

## Observabilidade

Registrar métricas sem payload:

- operação e módulo;
- duração;
- resultado por classe (`ok`, `oauth`, `scope`, `graphql`, `domain`,
  `transport`, `timeout`);
- renovação de token com sucesso/falha, nunca o token;
- versão/schema fingerprint usada pelo cliente;
- `transaction_id` de operação assíncrona;
- correlation ID entre request AMIgow, GraphQL e validação final.

Nunca registrar bearer token, client secret, query com valores, CDR completo,
caller ID, e-mail, senha de ramal/voicemail ou resposta de introspecção junto com
dados do PBX.

## Estratégia de testes

### Unitários

- cache/renovação concorrente do token;
- expiry e clock skew;
- GraphQL HTTP 200 com `errors`;
- resposta parcial;
- status de domínio falso;
- limite de body;
- timeout/cancelamento;
- retry só para queries;
- redaction de logs.

### Contrato

- schema snapshot sanitizado por versão do PBX;
- validação das operations usadas contra o snapshot;
- fixture para ausência de módulo/scope;
- teste de compatibilidade com os tipos Relay/paginação reais.

### Integração em PBX de desenvolvimento

- token read-only;
- query de sistema;
- query paginada do primeiro recurso;
- expiração e renovação do token;
- scope insuficiente falha fechado;
- nenhuma alteração em banco, dialplan ou Asterisk no marco read-only.

### Escrita, em marco separado

- objeto descartável aprovado;
- snapshot antes/depois;
- reconciliação por leitura;
- apply somente em janela autorizada;
- status assíncrono terminal;
- verificação AMI/Asterisk e rollback documentado.

## Ordem de rollout proposta

1. documentação e snapshot de capacidade;
2. aplicação OAuth M2M read-only criada pela GUI;
3. cliente OAuth/GraphQL desabilitado por padrão;
4. queries de sistema e módulos;
5. primeiro recurso read-only escolhido;
6. observabilidade e teste de falhas;
7. somente depois, nova decisão para mutation;
8. apply/reload permanece uma capacidade separada e opt-in.
