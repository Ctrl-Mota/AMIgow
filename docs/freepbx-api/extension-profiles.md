---
title: Perfis customizados na criação de ramais por GraphQL
status: verified-gap-and-proposal
last_verified: 2026-09-22
verified_host: safehouse_freepbx_dev
custom_module: extension_profiles
---

# Perfis customizados na criação de ramais por GraphQL

## Resposta executiva

Na implementação verificada, **não**. A mutation Core `addExtension` executa o
mesmo `Core::processQuickCreate` usado pela GUI e chega ao hook do módulo
`extension_profiles`, mas o schema GraphQL não aceita os campos
`extension_profile` e `condominio_id`. O hook recebe o perfil vazio e não aplica
as predefinições.

Não confundir três estados:

1. criar e persistir o ramal;
2. associar/aplicar o perfil customizado;
3. executar **Apply Config** para carregar a configuração no runtime.

O GraphQL Core atual faz o primeiro e marca `needreload`. Ele não faz o segundo
com o contrato disponível e não deve executar o terceiro implicitamente.

## Fluxos observados

### Quick Create pela GUI

```text
select extension_profile
        |
        +--> profiles.js copia defaults para os campos visíveis
        |
        v
POST quickcreate com extension_profile + condominio_id
        |
        v
Core::processQuickCreate cria device e user
        |
        v
hook Extension_profiles::processQuickCreate
        |
        +--> salva associação do perfil
        +--> grava defaults PJSIP na tabela sip
        +--> salva projeção extension -> condominio
        +--> gera o fragmento de configuração do módulo
        |
        v
Core marca needreload
```

### `addExtension` pelo GraphQL atual

```text
input addExtension (whitelist do Core)
        |
        |  extension_profile não existe no tipo de input
        |  condominio_id não existe no tipo de input
        v
Core::processQuickCreate cria device e user
        |
        v
hook Extension_profiles::processQuickCreate
        |
        +--> profileName = ""
        +--> condominioId = ""
        +--> nenhuma associação/default customizado é aplicado
        |
        v
Core marca needreload
```

Se o cliente tentar acrescentar `extension_profile` ao payload, a validação
GraphQL deve rejeitar o campo desconhecido antes de executar a mutation. Se o
campo for omitido, o ramal é criado sem o perfil.

## Evidência do baseline

| Evidência | Comportamento confirmado |
|---|---|
| `extension_profiles/module.xml:16-20` | Registra hooks para `getQuickCreateDisplay`, `processQuickCreate` e `delUser` do Core |
| `Extension_profiles.class.php:134-205` | Renderiza os selects `extension_profile` e `condominio_id` somente na UI |
| `assets/js/profiles.js:36-76` | Copia defaults para o formulário quando o operador troca o perfil |
| `Extension_profiles.class.php:208-231` | Só aplica perfil quando `$data['extension_profile']` está preenchido |
| `Extension_profiles.class.php:300-341` | Persiste a associação e grava defaults diretamente nas linhas PJSIP |
| `core/Api/Gql/Extensions.php:63` | `addExtension` chama `Core::processQuickCreate(..., $input)` |
| `core/Api/Gql/Extensions.php:463-526` | O input de criação é uma lista fechada e não contém os campos customizados |
| `core/Core.class.php:334-348` | O Core executa os hooks depois de criar device/user e chama `needreload()` |

Os caminhos acima são do filesystem do PBX de desenvolvimento em
`/var/www/html/admin/modules/`, verificados em 2026-09-22. O código oficial do
Core 17 também documenta a mutation e o Quick Create, mas a customização
`extension_profiles` é local e precisa ser verificada em cada instalação.

## Contrato recomendado

Não editar `core/Api/Gql/Extensions.php`: uma atualização do módulo Core pode
sobrescrever a mudança e o campo customizado passaria a pertencer ao scope
genérico de escrita de ramais.

Adicionar um provider GraphQL ao próprio módulo `extension_profiles`, com uma
mutation de caso de uso, por exemplo `createExtensionFromProfile`. Ela deve ter
scope de escrita próprio do módulo e uma whitelist pequena de argumentos:

```graphql
mutation AMIgowCreateExtensionFromProfile(
  $input: createExtensionFromProfileInput!
) {
  createExtensionFromProfile(input: $input) {
    status
    message
    extensionId
    needReload
    clientMutationId
  }
}
```

Input conceitual:

```json
{
  "extensionId": "1001",
  "name": "Portaria",
  "email": "",
  "profileName": "<nome-validado-no-servidor>",
  "condominioId": 123,
  "clientMutationId": "<id-de-correlacao>"
}
```

O nome e os tipos finais do schema só devem ser congelados depois de implementar
o provider e validar sua introspecção. O exemplo não é uma API já disponível.

### Comportamento obrigatório da mutation

1. Exigir ramal numérico e tecnologia PJSIP, pois o aplicador atual ignora
   outras tecnologias.
2. Resolver `profileName` no servidor e rejeitar perfil ausente; hoje o hook
   aceita nome desconhecido como no-op silencioso.
3. Validar `condominioId` e tratá-lo como projeção de um identificador cuja
   fonte de verdade continua sendo a SistemaSafeHouse.
4. Montar dados internos com `extension_profile` e `condominio_id` e chamar
   `Core::processQuickCreate` uma única vez. Isso reaproveita o ciclo oficial e
   o hook já existente.
5. Ler explicitamente o `status` retornado pelo Core; não converter o array
   inteiro em booleano.
6. Confirmar após a escrita: ramal existente, associação de perfil correta e
   defaults esperados persistidos.
7. Retornar `needReload`, mas não chamar `doreload` automaticamente.
8. Não repetir a mutation automaticamente quando a resposta for incerta.

Uma mutation única reduz o risco de criar o ramal e falhar antes de uma segunda
chamada para aplicar o perfil. Ainda assim, os writes internos do FreePBX e do
módulo não formam necessariamente uma transação SQL única; leitura de
reconciliação continua obrigatória.

## Alternativa de transição

Uma mutation `applyExtensionProfile` separada pode ser usada depois de
`addExtension`, mas cria um estado parcial possível: ramal existente sem perfil.
Ela só é aceitável com reconciliação explícita, idempotência por `extensionId`
e um procedimento de compensação. Para o AMIgow, a mutation única é preferível.

## Segurança e operação

- Criar scopes próprios de leitura/escrita para `extension_profiles`; não usar
  um token administrativo amplo.
- Nunca aceitar um mapa arbitrário de defaults vindo do AMIgow. O cliente envia
  apenas o nome; o PBX resolve a definição versionada localmente.
- Senha SIP não deve ser um segredo compartilhado dentro do perfil e nunca deve
  aparecer em log, resposta GraphQL ou documentação.
- A mutation apenas persiste e marca necessidade de reload. Apply Config é uma
  operação separada, com janela operacional, zero chamadas verificado via AMI e
  acompanhamento assíncrono até estado terminal.
- Um perfil desconhecido deve falhar fechado; não criar silenciosamente um
  ramal padrão quando o caller pediu um perfil.

## Teste de aceite no PBX de desenvolvimento

Usar um ramal descartável, depois de autorização para mutation:

1. introspectar a mutation customizada e confirmar o scope exato;
2. criar o ramal com um perfil conhecido e `clientMutationId` único;
3. consultar o ramal pelo Core GraphQL;
4. conferir de forma sanitizada a associação em `extension_profiles` e os
   keywords esperados, sem exibir secrets;
5. confirmar a projeção de condomínio quando enviada;
6. testar perfil inexistente e garantir que nenhum ramal seja criado;
7. testar reenvio e resultado incerto sem retry cego;
8. verificar `fetchNeedReload`;
9. somente numa janela autorizada, executar Apply Config e validar Asterisk,
   AMI e registro PJSIP.

Até esse teste passar, a criação por GraphQL não deve ser considerada
equivalente ao Quick Create customizado da GUI.
