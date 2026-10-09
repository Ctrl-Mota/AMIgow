---
title: Integração AMIgow com a PBX API do FreePBX
status: discovery
last_verified: 2026-09-22
freepbx_baseline: "17.0.33"
api_module_baseline: "17.0.9"
---

# Integração AMIgow com a PBX API do FreePBX

Este diretório é o caderno técnico da integração do AMIgow com a API nativa do
FreePBX. Ele separa fatos confirmados, desenho recomendado e trabalho ainda não
autorizado para que novas integrações não sejam construídas diretamente sobre o
banco do FreePBX ou sobre suposições tiradas de uma versão diferente do PBX.

## Estado atual

- A pesquisa e o inventário read-only foram concluídos em 2026-09-22.
- Nenhum cliente GraphQL foi implementado no AMIgow.
- Nenhuma aplicação OAuth, credencial, mutation, reload ou alteração no PBX foi
  criada por esta etapa.
- O primeiro piloto deve ser somente leitura. A escolha do primeiro recurso de
  negócio ainda está aberta.

## Conclusão curta

GraphQL é uma boa opção para **configuração persistente e consultas
administrativas** do FreePBX. Ele não substitui a AMI.

| Necessidade | Interface preferida | Motivo |
|---|---|---|
| Eventos de chamada, canais e filas em tempo real | AMI | Fluxo assíncrono e estado operacional do Asterisk |
| Ações imediatas sobre chamadas | AMI, com allowlist | Semântica de runtime; já existe no AMIgow |
| Ramais, DIDs, ring groups, voicemail e preferências | GraphQL | Estado persistido e governado pelos módulos FreePBX |
| Estado do FreePBX, módulos e necessidade de reload | GraphQL | Contrato do módulo `framework` |
| CDR histórico | GraphQL como candidato | Evita acoplamento direto ao schema SQL, mas requer benchmark e revisão de PII |
| Filas no PBX validado | AMI + metadados atuais | O módulo `queues` está instalado, mas não publica provider/escopo GraphQL |
| Trunks genéricos no PBX validado | Não assumir suporte GraphQL | O provider `core/Api/Gql/Trunks.php` está comentado e não há escopo de trunk |

## Fronteira arquitetural

O AMIgow continua sendo um processo local por PBX. A nova integração deve usar
o mesmo princípio de proximidade da AMI, mas através do endpoint HTTPS do
FreePBX:

```text
SistemaSafeHouse
      |
      | contrato de produto / fonte de verdade
      v
   AMIgow
      |\
      | \__ AMI 127.0.0.1:5038
      |
      \____ FreePBX OAuth2 + GraphQL (HTTPS)
```

GraphQL não deve transformar o FreePBX em uma segunda fonte de verdade para
dados de condomínio, morador ou configuração de produto. Ele serve para ler ou
alterar objetos que pertencem ao próprio PBX.

## Como usar este caderno

1. Leia [capability-map.md](capability-map.md) antes de escolher um recurso.
2. Para criação de ramais com as predefinições locais, leia
   [extension-profiles.md](extension-profiles.md). A mutation Core padrão não
   transporta o perfil customizado.
3. Use [integration-design.md](integration-design.md) para implementar o
   cliente Go sem misturar AMI, GraphQL e acesso SQL.
4. Execute [discovery-runbook.md](discovery-runbook.md) em cada PBX antes de
   habilitar uma operação; o schema varia conforme módulos e versões.
5. Consulte [references.md](references.md) para as fontes oficiais e limitações
   conhecidas.

## Regras que não devem ser quebradas

1. O schema introspectado no PBX alvo prevalece sobre exemplos da internet.
2. A aplicação OAuth começa com scopes read-only e de menor privilégio.
3. Token e client secret nunca entram em Git, `config.json`, logs, query string
   ou respostas da API do AMIgow.
4. HTTP 200 não significa sucesso GraphQL: sempre processar `errors`, `data` e
   os campos de domínio `status`/`message`.
5. Mutations não recebem retry automático.
6. `doreload` não faz parte do cliente genérico. Aplicar configuração é uma
   operação separada, assíncrona e governada.
7. Antes de qualquer apply/reload, validar chamadas ativas pela AMI e acompanhar
   a transação GraphQL até o estado terminal.
8. Respostas com CDR, ramais, caller ID ou gravações são dados sensíveis; não
   persistir payloads completos como evidência.

## Baseline verificado

O snapshot read-only de `safehouse_freepbx_dev` em 2026-09-22 encontrou:

- FreePBX Framework `17.0.33`;
- PBX API `17.0.9`, habilitada;
- Core `17.0.18.54`;
- Asterisk `22.8.2`;
- 36 raízes de scope GraphQL ativas;
- token endpoint `https://<pbx>/admin/api/api/token`;
- GraphQL endpoint `https://<pbx>/admin/api/api/gql`;
- HTTP local recusado com 403, enquanto HTTPS alcança o módulo API.

Este baseline é evidência do PBX de desenvolvimento, não garantia para outras
VPS. Repita a descoberta antes de usar o catálogo em outro PBX.

## Próximo marco sugerido

Criar pela GUI uma aplicação **Machine-to-Machine** exclusiva para o AMIgow,
com scopes read-only mínimos, capturar uma introspecção sanitizada e implementar
um primeiro adaptador de consulta. Nenhuma mutation deve entrar no mesmo marco.
