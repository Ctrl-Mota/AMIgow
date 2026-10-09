---
title: Referências da PBX API e GraphQL
status: active
last_checked: 2026-09-22
---

# Referências da PBX API e GraphQL

As páginas abaixo foram verificadas em 2026-09-22. A documentação pública é
referência de intenção e exemplos; o schema introspectado do PBX alvo continua
sendo a fonte executável.

## Documentação Sangoma/FreePBX

- [PBX API Documentation](https://sangomakb.atlassian.net/wiki/spaces/FP/pages/10289746/PBX+API)
  — introdução oficial; GraphQL é a opção preferida e o suporte depende de cada
  módulo.
- [PBX GUI - API](https://sangomakb.atlassian.net/wiki/spaces/PG/pages/24182986/)
  — visão geral do módulo API e navegação para autenticação, scopes e GraphQL.
- [API Authentication](https://sangomakb.atlassian.net/wiki/spaces/PG/pages/25722906/API+Authentication)
  — grants OAuth2 suportados.
- [API Applications](https://sangomakb.atlassian.net/wiki/spaces/PG/pages/26148878/PBX+GUI+-+API+Applications)
  — criação de aplicações; Machine-to-Machine corresponde ao client credentials.
- [API Scope Visualizer](https://sangomakb.atlassian.net/wiki/spaces/PG/pages/26181647/PBX+GUI+-+API+Scope+Visualizer)
  — fonte visual dos scopes válidos no PBX instalado.
- [API GraphQL](https://sangomakb.atlassian.net/wiki/spaces/PG/pages/25886741/PBX+GUI+-+API+GraphQL)
  — GraphQL Explorer e Documentation integrados à GUI.
- [GraphQL API / introspection](https://sangomakb.atlassian.net/wiki/spaces/FP/pages/11960339/)
  — exemplo oficial de consulta `__schema`.
- [GraphQL PBX APIs Documentation](https://sangomakb.atlassian.net/wiki/spaces/PG/pages/25755702/)
  — catálogo agregado de operações por módulo; útil para descoberta, não para
  congelar o contrato.
- [Core Module GraphQL APIs](https://sangomakb.atlassian.net/wiki/spaces/PG/pages/25722965/Core+Module+GraphQL+APIs)
  — ramais, devices, DIDs/inbound routes e advanced settings.
- [Framework Module GraphQL APIs](https://sangomakb.atlassian.net/wiki/spaces/PG/pages/26116169/PBX+GUI+-+Framework+Module+GraphQL+APIs)
  — sistema, módulos, `fetchNeedReload`, `doreload` e status assíncrono.
- [CDR Module GraphQL APIs](https://sangomakb.atlassian.net/wiki/spaces/PG/pages/26083384/CDR+Module+GraphQL+APIs)
  — CDR individual e paginado.
- [GraphQL Provisioning Tutorial](https://sangomakb.atlassian.net/wiki/spaces/FCD/pages/10354832/FreePBX+GraphQL+Provisioning+Tutorial)
  — fluxo completo de app M2M, token, consulta, mutation e apply assíncrono.

## Código oficial FreePBX 17

- [PBX API module.xml, release/17.0](https://github.com/FreePBX/api/blob/release/17.0/module.xml)
  — versão, tabelas OAuth e changelog de correções de autenticação/sanitização.
- [PBX API Api.class.php, release/17.0](https://github.com/FreePBX/api/blob/release/17.0/Api.class.php)
  — descoberta de scopes e disparo assíncrono de operações/reload.
- [Framework module.xml, release/17.0](https://github.com/FreePBX/framework/blob/release/17.0/module.xml)
  — changelog, inclusive correções de controle de acesso GraphQL.
- [Core GraphQL providers, release/17.0](https://github.com/FreePBX/core/tree/release/17.0/Api/Gql)
  — implementação atual das operações Core.
- [Core Extensions GraphQL provider, release/17.0](https://github.com/FreePBX/core/blob/release/17.0/Api/Gql/Extensions.php)
  — campos aceitos por `addExtension`/`updateExtension` e chamada ao Quick
  Create do Core.
- [Core class, release/17.0](https://github.com/FreePBX/core/blob/release/17.0/Core.class.php)
  — implementação de `processQuickCreate`, incluindo execução dos hooks dos
  módulos e marcação de `needreload`.
- [Core Trunks provider, release/17.0](https://github.com/FreePBX/core/blob/release/17.0/Api/Gql/Trunks.php)
  — no baseline e no branch oficial consultado, o provider funcional está
  comentado; não assumir CRUD genérico de trunks.
- [Ring Groups GraphQL provider, release/17.0](https://github.com/FreePBX/ringgroups/blob/release/17.0/Api/Gql/Ringgroups.php)
  — exemplo de provider modular e `needreload()` após mutation.

## Limitações públicas úteis para teste

Issues não substituem documentação nem provam o comportamento de toda versão,
mas identificam casos que o plano de testes deve cobrir:

- [Core #106 - inbound route não marca need reload](https://github.com/FreePBX/core/issues/106)
- [Core #162 - updateExtension e campo name](https://github.com/FreePBX/core/issues/162)
- [FreePBX issue-tracker #626 - validação de Ring Groups GraphQL](https://github.com/FreePBX/issue-tracker/issues/626)

## Evidência local do baseline

O inventário de 2026-09-22 foi produzido em `safehouse_freepbx_dev` com:

```text
fwconsole --version
fwconsole ma list
FreePBX::Api()->getFlattenedScopes()
find ... -path '*/Api/Gql/*.php'
asterisk -rx 'core show version'
probes HTTP/HTTPS sem credenciais
```

Nenhuma aplicação OAuth, token, mutation, reload ou alteração de módulo foi
criada nessa coleta.
