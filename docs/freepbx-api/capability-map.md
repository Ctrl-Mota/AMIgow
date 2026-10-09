---
title: Mapa de capacidades GraphQL do FreePBX
status: verified-baseline
last_verified: 2026-09-22
verified_host: safehouse_freepbx_dev
---

# Mapa de capacidades GraphQL do FreePBX

## Como interpretar este catálogo

Há três níveis de evidência e eles não são equivalentes:

1. **Catálogo oficial:** mostra operações conhecidas, mas pode estar atrasado ou
   descrever outro conjunto de módulos.
2. **Scope válido:** confirma que o módulo registra permissões no PBX alvo, mas
   não garante que exista uma query/mutation útil para cada permissão.
3. **Schema introspectado:** é a fonte executável para nomes, argumentos e tipos
   disponíveis naquele PBX. Este nível ainda depende da criação de uma aplicação
   OAuth read-only.

Portanto, nenhum recurso deve ser implementado usando apenas o nome visto na
documentação pública.

## Baseline do PBX de desenvolvimento

| Componente | Versão/estado observado |
|---|---|
| FreePBX Framework | `17.0.33`, habilitado |
| PBX API | `17.0.9`, habilitado |
| Core | `17.0.18.54`, habilitado |
| Asterisk | `22.8.2` |
| Transporte local | HTTP recusado; HTTPS alcança o módulo API |
| Raízes de scope GraphQL | 36 |

O módulo API 17.0.9 contém correções posteriores às versões que registraram
falha de autenticação e sanitização de comandos. O Framework 17.0.33 é posterior
à correção de controle de acesso GraphQL registrada nas versões 17.0.31/32. Isso
não elimina a necessidade de menor privilégio.

## Recursos mais úteis ao AMIgow

| Domínio | Capacidades documentadas | Scope inicial candidato | Situação no baseline | Direção |
|---|---|---|---|---|
| Sistema | Versões, Asterisk, AMI, banco, modo da GUI | `gql:framework:read:system` | Scope ativo | Bom primeiro smoke test |
| Módulos | Lista/status de módulos e transações assíncronas | `gql:framework:read:modules` | Scope ativo | Útil para compatibilidade e diagnóstico |
| Ramais | Listar, consultar, criar, atualizar e remover | `gql:core:read:extension` | Scopes read/write ativos | Melhor candidato funcional após o smoke test |
| Dispositivos | Listar/consultar e manter Core Devices | `gql:core:read:device` | Scope read ativo; write usa `devices` no plural | Exige snapshot do schema para evitar erro de scope |
| DIDs/rotas de entrada | Consultar e manter DIDs/inbound routes | `gql:core:read:did` | Scopes read/write ativos | Validar nomes reais por introspecção |
| CDR | `fetchAllCdrs`, `fetchCdr` | `gql:cdr:read` | Scope ativo | Candidato a substituir acesso SQL, após benchmark e revisão de PII |
| Ring groups | CRUD e consulta | `gql:ringgroups:read` | Scope ativo | Persistente; não é estado de chamada ao vivo |
| Voicemail | Consultar, habilitar e desabilitar | `gql:voicemail:read` | Scope ativo | Separar leitura de mutation |
| Preferências do ramal | Call forward, call waiting, DND, find-me/follow-me | Scope read do módulo específico | Scopes ativos | Bons recursos isolados por usuário/ramal |
| Mídia do PBX | Recordings e music-on-hold | Scope read do módulo específico | Scopes ativos | Respostas podem conter caminhos/dados sensíveis |
| SIP global | NAT e WebSocket globais | `gql:sipsettings:read` | Scope ativo | Alto impacto; não incluir no piloto de escrita |
| Apply Config | `fetchNeedReload`, `doreload`, `fetchApiStatus` | Framework read/write separados | Disponível na documentação e no código | Manter fora do cliente genérico |

### Operações oficiais relevantes

Os nomes abaixo são referências para a introspecção, não um contrato congelado:

- Core: `fetchAllExtensions`, `fetchExtension`, `addExtension`,
  `updateExtension`, `deleteExtension`, `createRangeofExtension`;
- Core/DID: `fetchAllInboundRoutes`, `fetchInboundRoute`,
  `addInboundRoute`, `updateInboundRoute`, `removeInboundRoute`;
- Core Device: `fetchAllCoreDevice`, `fetchCoreDevice`, `addCoreDevice`,
  `updateCoreDevice`, `deleteCoreDevice`;
- Framework: `fetchNeedReload`, `fetchAsteriskDetails`, `fetchDBStatus`,
  `fetchInstalledModules`, `fetchModuleStatus`, `fetchApiStatus`, `doreload`;
- CDR: `fetchAllCdrs`, `fetchCdr`;
- Ring groups: `fetchAllRingGroup`, `fetchRingGroup`, `addRingGroup`,
  `updateRingGroup`, `deleteRingGroup`;
- Voicemail: `fetchVoiceMail`, `enableVoiceMail`, `disableVoiceMail`.

Há operações administrativas de amplo poder, como `fwconsoleCommand`, mudanças
de módulos, firewall, certificados, backup/restore e configuração do sistema.
Elas não pertencem ao primeiro token do AMIgow.

## Inventário de scopes ativos em 2026-09-22

### Telefonia e recursos do usuário

| Raiz | Natureza |
|---|---|
| `gql:core` | ramais, devices, DIDs e outros tipos expostos pelo Core |
| `gql:cdr` | histórico CDR |
| `gql:voicemail` | voicemail por ramal |
| `gql:ringgroups` | ring groups |
| `gql:callforward` | encaminhamento |
| `gql:callwaiting` | chamada em espera |
| `gql:donotdisturb` | DND |
| `gql:findmefollow` | find-me/follow-me |
| `gql:announcement` | announcements |
| `gql:callback` | callbacks |
| `gql:blacklist` | blacklist |
| `gql:music` | music-on-hold |
| `gql:parking` | estacionamento de chamadas |
| `gql:recordings` | gravações de sistema |
| `gql:sipsettings` | configurações SIP globais |

### Plataforma e operação

| Raiz | Natureza |
|---|---|
| `gql:framework` | sistema, módulos, reload e transações |
| `gql:dashboard` | dados do dashboard FreePBX |
| `gql:pm2` | processos administrados pelo PM2 |
| `gql:backup` | configuração e restore de backup |
| `gql:certman` | certificados e CSR |
| `gql:filestore` | destinos de arquivo |
| `gql:firewall` | configuração de firewall |
| `gql:sysadmin` | licença, hostname, portas e SSL do Sysadmin |

### Provisionamento e módulos comerciais

| Raiz | Natureza |
|---|---|
| `gql:endpoint` | Endpoint Manager/DPMA |
| `gql:sipstation` | SIPStation |
| `gql:broadcast` | campanhas de broadcast |
| `gql:areminder` | appointment reminder |
| `gql:adv_recovery` | Advanced Recovery |
| `gql:sangomaconnect` | Sangoma Connect |
| `gql:scribe` | transcrição |
| `gql:sms` | SMS |
| `gql:voipinnovations` | integração VoIP Innovations |
| `gql:parkpro` | Parking Pro |

### Outros providers

`gql:allowlist`, `gql:arimanager` e `gql:hotelwakeup` também estão ativos.

O inventário foi obtido de `FreePBX::Api()->getFlattenedScopes()` no host alvo.
Ele deve ser regenerado após atualização ou alteração de módulos.

## Scopes granulares confirmados

Além dos scopes amplos `<módulo>:read` e `<módulo>:write`, o baseline publicou:

```text
gql:core:read:device
gql:core:read:did
gql:core:read:extension
gql:core:write:devices
gql:core:write:did
gql:core:write:extension
gql:endpoint:read:endpoint
gql:endpoint:write:endpoint
gql:firewall:read:firewall
gql:firewall:write:firewall
gql:framework:read:modules
gql:framework:read:system
gql:framework:write:modules
gql:framework:write:system
gql:sangomaconnect:read:sangomaconnect
gql:scribe:read:transcription
```

O plural inconsistente em `gql:core:write:devices` é real no baseline. Não
normalizar nem inventar scopes no cliente; copiar o valor do Scope Visualizer.

## Lacunas confirmadas

### Queues

O módulo `queues` 17.0.4 está habilitado, mas não há diretório/provider
`queues/Api/Gql` nem raiz `gql:queues`. Para o PBX verificado:

- eventos, membros, callers e métricas continuam na AMI;
- os metadados que o AMIgow ainda consulta no banco não podem ser migrados para
  GraphQL sem outro provider;
- uma página pública que mencione “a maioria dos módulos” não autoriza assumir
  suporte a filas.

### Trunks genéricos

O arquivo `core/Api/Gql/Trunks.php` existe, mas seu conteúdo funcional está
comentado e o Scope Visualizer não publicou scope de trunk. SIPStation tem API
própria, mas não equivale a CRUD genérico de trunks PJSIP.

Não usar GraphQL para criar ou alterar trunks até que o schema real publique uma
operação específica e ela seja validada em um ambiente de teste. O procedimento
operacional vigente para trunk de teste continua sendo a GUI.

### Perfis customizados de ramal

O módulo local `extension_profiles` está integrado ao Quick Create por hooks de
`getQuickCreateDisplay` e `processQuickCreate`. A mutation Core
`addExtension` também chama `Core::processQuickCreate`, portanto o hook do
módulo é executado tanto pela GUI quanto por GraphQL.

Isso, sozinho, não transporta o perfil. No baseline atual:

- o formulário da GUI envia `extension_profile` e `condominio_id`;
- o JavaScript da GUI também copia os defaults do perfil para os campos do
  formulário;
- o input GraphQL de `addExtension` não declara `extension_profile` nem
  `condominio_id`;
- o GraphQL rejeita campos que não pertencem ao input antes de chamar o Core;
- sem esses campos, o hook roda com perfil vazio e não persiste nem aplica as
  predefinições.

Consequentemente, `addExtension` cria um ramal padrão e marca necessidade de
reload, mas **não equivale ao Quick Create com um perfil selecionado**. O
contrato e a alternativa recomendada estão em
[extension-profiles.md](extension-profiles.md).

### Runtime de chamadas

GraphQL não substitui os eventos AMI, não é um stream e não deve ser usado para
polling de canais/filas em alta frequência.

## Riscos de contrato observados

- A Sangoma avisa que a API ainda evolui e pode mudar.
- Há issues públicas em versões 16/17 sobre `updateExtension`, validação de ring
  groups e inbound route não marcando `needreload`.
- Algumas mutations retornam HTTP 200 com `data`, mas sinalizam falha em
  `status: false` ou retornam campos nulos.
- `doreload` apenas inicia uma transação assíncrona; sucesso inicial não prova
  `Reload Complete`.
- No baseline, uma chamada GraphQL sem bearer token respondeu HTTP 500 com uma
  negação OAuth no corpo, em vez de um 401 limpo. Classificação de erro não pode
  depender somente do status HTTP.

## Critério de disponibilidade para uma integração

Um recurso só é considerado disponível ao AMIgow quando todos os itens passam:

1. módulo instalado e habilitado;
2. scope exato visível no Scope Visualizer;
3. query/mutation presente na introspecção do PBX alvo;
4. smoke test com uma aplicação OAuth de menor privilégio;
5. comportamento de erro e compatibilidade registrado;
6. para mutation, leitura posterior confirma o estado persistido;
7. quando necessário, apply/reload separado termina com sucesso e o Asterisk
   continua saudável.
