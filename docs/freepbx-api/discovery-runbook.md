---
title: Runbook de descoberta e validação da PBX API
status: active
last_verified: 2026-09-22
---

# Runbook de descoberta e validação da PBX API

Este procedimento produz um inventário reproduzível sem alterar Asterisk,
dialplan, trunks, rotas ou módulos. As etapas de escrita ficam em um gate
separado no final.

## 1. Pré-condições

- Confirmar que o host é o PBX/ambiente pretendido.
- Começar em desenvolvimento.
- Não executar `fwconsole reload`, restart, mutation ou criação de objeto durante
  a descoberta.
- Não copiar token, secret, respostas com PII ou dumps do schema autenticado para
  logs de terminal compartilhados.
- Usar a GUI para criar e gerir a aplicação OAuth.

## 2. Inventariar versões e módulos

Comandos read-only:

```bash
sudo -n fwconsole --version
sudo -n fwconsole ma list
sudo -n -u asterisk /usr/sbin/asterisk -rx 'core show version'
```

Registrar apenas:

- versão do Framework;
- versão/estado dos módulos `api`, `core`, `framework` e do recurso desejado;
- versão do Asterisk;
- data e ambiente.

Não instalar/atualizar módulo como parte desta etapa.

## 3. Ver providers presentes

Um arquivo não prova que o provider está ativo, mas ajuda a encontrar lacunas:

```bash
find /var/www/html/admin/modules /var/www/html/admin/libraries \
  -type f -path '*/Api/Gql/*.php' -print | sort
```

Verificações importantes:

- módulo instalado sem `Api/Gql` provavelmente não oferece GraphQL;
- provider com código comentado não está disponível;
- arquivos de teste não são providers;
- providers comerciais dependem de módulo/licença ativos.

## 4. Extrair scopes válidos

O Scope Visualizer em **Connectivity > API** é a interface operacional
preferida. Para uma auditoria read-only no shell, o mesmo inventário pode ser
obtido do FreePBX:

```bash
sudo -n php <<'PHP'
<?php
include '/etc/freepbx.conf';
$scopes = FreePBX::Api()->getFlattenedScopes();
ksort($scopes);
foreach ($scopes as $key => $meta) {
    if (str_starts_with($key, 'gql:')) {
        echo $key, PHP_EOL;
    }
}
PHP
```

O comando lista nomes, não credenciais. Compare o resultado com o snapshot em
[capability-map.md](capability-map.md).

## 5. Confirmar endpoints sem autenticar

Os URLs são exibidos na aba **API URL List** da GUI. O formato usual é:

```text
https://<pbx-fqdn>/admin/api/api/token
https://<pbx-fqdn>/admin/api/api/gql
```

Uma resposta de erro por ausência de credencial confirma somente roteamento. Ela
não valida scopes, schema nem operação. Não use `-k` como configuração do
AMIgow; corrija DNS/certificado.

No baseline de 2026-09-22:

- HTTP retornou 403;
- HTTPS do token endpoint retornou erro OAuth de request incompleto;
- GraphQL sem bearer retornou HTTP 500 com negação OAuth no corpo.

Esses códigos são comportamento observado, não contrato portátil.

## 6. Criar a aplicação OAuth pela GUI

Em **Connectivity > API**:

1. abrir **Scope Visualizer**;
2. selecionar apenas scopes read-only necessários;
3. copiar a string exata gerada;
4. abrir **Applications** e criar **Machine-to-Machine App**;
5. nomear por serviço e ambiente, por exemplo `amigow-dev-inventory`;
6. colar os Allowed Scopes exatos;
7. copiar o client secret uma única vez para o secret store/arquivo protegido;
8. não colocar ID/secret em ticket, Markdown, Git ou chat.

Scopes iniciais sugeridos:

```text
gql:framework:read:system
gql:framework:read:modules
gql:core:read:extension
```

Adicionar `gql:cdr:read` somente a uma credencial/caso de uso que precise de CDR.
Não incluir `write`, `framework:write`, firewall, backup, certman, sysadmin ou
sipsettings no primeiro app.

## 7. Obter token

O request usa HTTP Basic e form encoding:

```http
POST /admin/api/api/token HTTP/1.1
Host: <pbx-fqdn>
Authorization: Basic base64(client_id:client_secret)
Content-Type: application/x-www-form-urlencoded

grant_type=client_credentials&scope=<scopes URL-encoded>
```

Resposta esperada:

```json
{
  "token_type": "Bearer",
  "expires_in": 3600,
  "access_token": "<redacted>"
}
```

Não persistir a resposta completa. Para teste manual, prefira o GraphQL Explorer
da GUI; para automação, use o cliente Go e o secret store, evitando credenciais
na linha de comando.

## 8. Introspectar o schema real

O GraphQL Explorer/Documentation do próprio PBX pode carregar o schema com os
scopes escolhidos. Uma consulta mínima para inventariar raízes é:

```graphql
query AMIgowSchemaRoots {
  __schema {
    queryType {
      fields {
        name
        description
        args {
          name
          type {
            kind
            name
            ofType {
              kind
              name
            }
          }
        }
      }
    }
    mutationType {
      fields {
        name
        description
        args {
          name
          type {
            kind
            name
            ofType {
              kind
              name
            }
          }
        }
      }
    }
  }
}
```

Para gerar tipos, capture também os input/object types alcançados pelas
operações usadas. Salvar somente o schema, nunca respostas de dados.

Nome recomendado para um snapshot futuro:

```text
docs/freepbx-api/snapshots/freepbx-17.0.33-api-17.0.9-YYYY-MM-DD.schema.json
```

Antes de versionar, verificar que o arquivo contém apenas introspecção e não
headers, bearer token, URL privada com credenciais ou resultados do PBX.

## 9. Smoke tests read-only

Executar nesta ordem:

1. `fetchAsteriskDetails` com scope de system;
2. consulta de módulos com scope de modules;
3. primeira página do recurso escolhido, com seleção mínima de campos;
4. repetir após expiração/renovação de token;
5. executar uma operação sem o scope correspondente e confirmar falha fechada;
6. confirmar que nenhuma operação marcou `needreload`.

Não guardar respostas com ramal, caller ID, e-mail ou CDR como evidência. Guarde
apenas status, contagem, tempo, versão e fingerprint do schema.

## 10. Registrar compatibilidade

Para cada operação aprovada, preencher:

```text
PBX/versões:
Módulo/provider:
Operation name:
Scope mínimo:
Tipo de query/mutation:
Entrada relevante:
Saída relevante:
Paginação:
Erros observados:
Marca needreload:
Requer apply:
Leitura de reconciliação:
Dados sensíveis:
Teste executado/data:
```

## 11. Gate para mutations

Não avançar para escrita até existir:

- recurso e objetivo de negócio escolhidos;
- scope write mínimo conhecido;
- objeto descartável ou plano de reversão;
- leitura antes/depois;
- regra para resultado incerto sem retry cego;
- definição de quem pode solicitar apply;
- janela operacional e verificação de zero chamadas;
- monitoramento de `transaction_id` quando assíncrono;
- aceite de que o módulo/versão foi testado.

Operações proibidas no primeiro piloto:

- `doreload` e `fwconsoleCommand`;
- instalar, remover, habilitar ou atualizar módulos;
- firewall/allowlist;
- certificado, hostname, portas ou licença;
- backup/restore;
- SIP/NAT/WSS global;
- DPMA com restart;
- criação/alteração de trunk;
- qualquer mutation de produção.

## 12. Validação final da descoberta

- [ ] Versões e módulos registrados.
- [ ] Scope Visualizer conferido.
- [ ] App M2M read-only criado pela GUI.
- [ ] Segredo fora do repositório e logs.
- [ ] Schema introspectado e sanitizado.
- [ ] Query de sistema passou.
- [ ] Query do recurso passou com seleção mínima.
- [ ] Scope insuficiente falhou fechado.
- [ ] Nenhuma mudança/reload ocorreu.
- [ ] Documento de compatibilidade atualizado.
