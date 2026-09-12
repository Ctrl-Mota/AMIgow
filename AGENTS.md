# AMIgow — ponte entre Asterisk e a API

Microserviço Go que roda **dentro da VPS de cada PBX**. Tem **três** papéis, e confundi-los é a origem da maior parte dos erros de diagnóstico:

1. **Proxy síncrono do dialplan** — o Lua do FreePBX chama `/dynamic-resolver` e `/open-gate` e **espera resposta**. Caminho crítico do interfone.
2. **Emissor de webhooks** para a `SistemaSafeHouse` a partir de eventos AMI.
3. **Projeção de estado de filas** de call center, com métricas e alertas.

Conhecimento institucional: `../engineering-context/systems/amigow.md` (auditada em 2026-09-03, `confidence: confirmed`), `systems/freepbx.md`, `concepts/sip.md`.

## Comandos

```bash
go build ./cmd
go test ./...      # testes só em internal/ami
go vet ./...
```

Deploy real é **swap manual de binário**: cross-compile `GOOS=linux`, `scp` para `~/amigow/`, `systemctl stop amigow` → `cp` → `start`. Sem versionamento, health gate ou rollback.

**O caminho Docker está morto**: `Dockerfile` usa `golang:1.21-alpine` contra `go 1.24.0` no `go.mod` — não compila. O `docker-compose.yml` referencia `CONFIG_API_URL`, cuja leitura está comentada no código.

## Restrições duras

1. **`webhooks` está aninhado em `ami_server`, não é chave de topo.**
   ```
   ami_server: { host, port, username, password, webhooks: [ {url, timeout_seconds, events_filter} ] }
   ```
   Errar o aninhamento produz **zero webhooks, sem nenhum erro**: `shouldSendEvent` é fail-closed para filtro vazio (`sender.go:41-43`) e o `/health` continua respondendo `ok`. Este é o erro de configuração mais caro do repositório.
2. **`timeout_seconds` não tem default.** Omitido = `timeout: 0` = **sem timeout**; um endpoint pendurado prende a goroutine para sempre.
3. **As rotas ficam SEMPRE na raiz.** `base_path` é apenas metadado do OpenAPI e log — não monta o router sob prefixo. Qualquer nginx sem a barra final no `proxy_pass` quebra tudo com 404, e mexer no `base_path` não conserta.
4. **A porta 8080 é hardcoded** (`cmd/main.go:352`). Não é configurável por config nem env.
5. **O dialplan Lua fala direto com `localhost:8080`**, sem nginx (`freepbx/portaria_autonoma.lua:44`). Mudar porta ou prefixo aqui **não** é percebido por ele.
6. **Não commite `config.json`** (senha AMI e do MySQL) nem `ami_stream_asterisk-01.log`. O `.gitignore` já cobre — `git ls-files` lista 44 arquivos e nenhum deles é binário, log ou config.
7. **Ao adicionar rota, garanta que o path não contenha `/docs`, `/health`, `/openapi.json` ou `/webhooks/schema` em nenhuma posição.** `ValidateAPIKey` usa `strings.Contains`, não prefixo (`middleware.go:15-18`) — uma rota assim **nasce sem autenticação**.

## Durabilidade dos webhooks: at-most-once (auditado)

**Uma tentativa. Sem retry, sem persistência, sem dead-letter.** O retry existe **comentado** em `sender.go:71-77`. O README já admitia: *"Treat delivery as at-most-once"*.

Quatro pontos independentes de perda:

1. **Drop no fan-out** — canal cheio (buffer 1000) descarta com `select`/`default` (`cmd/main.go:389-406`).
2. **Falha de entrega** — erro de transporte retorna `false` **sem logar** (log comentado em `sender.go:108`). O único vestígio é `"[WEBHOOK] Tentando retry para %s"` — **mensagem falsa, não existe retry**.
3. **Restart** — shutdown não faz drain; até 2000 eventos em trânsito morrem (`cmd/main.go:369-386`).
4. **`events_filter` vazio** — silêncio total, ver restrição 1.

**Não reformule isso como "portão não abre".** O caminho portão/resolver é **HTTP síncrono**: falha ali é imediata e audível para o morador (`pls-try-call-later`). O risco dos webhooks é **perda silenciosa de histórico e estado**, sem reconciliação possível — `queues/reconcile.go` corrige estado interno de filas, nunca reenvia webhook.

**Antes de investir em retry**, resolva o desconhecido nº 1 da nota do Vault: *o push do morador (`contact.type == "app"`) depende do webhook `invite`?* Se depender, at-most-once significa app que não toca, e a prioridade muda.

## Eventos AMI

**O dispatch não é `switch`.** É uma cadeia de predicados em ordem com early-return, em `internal/ami/events.go:14-98`, que traduz o evento AMI para um **nome canônico interno**. São 24 tipos canônicos.

Se você grepar `case "` e achar `Hangup`, `Status`, `CoreStatus`, `Ping`, `Logoff`, `Command` — **isso é `actions.go`, o dispatch de ações de SAÍDA**, não de eventos consumidos. Erro fácil de cometer.

Armadilhas:

- **`hangup` mascara `missed`**: `isHangupEvent` vem antes de `isMissedCallEvent`. Chamada não atendida que gere só `Hangup` sai como `hangup`; olhe `cause` (17/18/19/21).
- **`raw_data` não é cru**: `buildEvent` filtra por allowlist de ~62 campos (`events.go:230-247`). Adicionar campo ao consumo **exige recompilar**.
- **Chaves repetidas são achatadas**: `Get` devolve só o primeiro valor (`parser.go:26-32`).
- Todo evento fora da cadeia é **silenciosamente descartado**.

## Segurança — leia antes de expor qualquer coisa

O serviço faz bind em **`0.0.0.0:8080`** e autentica por header **`X-API-Key`** com igualdade simples de string. **Sem allowlist de IP, sem TLS, sem timeouts de servidor, sem rate limit, sem recover.**

**A key equivale a comando arbitrário no PBX**: `POST /action` com `Action: Command` executa CLI do Asterisk (`actions.go:136-146`); `PATCH /tip` roda `/opt/amigow/tip.sh`, com sudoers NOPASSWD sugerido para `fwconsole` e `fail2ban-client` (**firewall**). `/docs`, `/openapi.json` e `/schemas/*` são **públicos** — inventário completo da superfície de comando para quem alcançar a porta.

"Exclusivo para a API" é propriedade do firewall/nginx da VPS, **não do serviço**. Verifique que esse firewall existe antes de assumir qualquer coisa.

Duas contradições documentais ativas: as descrições OpenAPI de `/dynamic-resolver` e `/open-gate` dizem *"sem autenticação (uso interno)"*, mas ambas **exigem** a key (a linha que as tornava públicas está comentada em `cmd/main.go:140`). Quem ler o `/docs` constrói a integração errada.

**Pendência**: `ami_stream_asterisk-01.log` contém **39 linhas com o campo `Secret:`** do handshake AMI. Rotacionar a senha AMI e apagar o arquivo.

## Bugs ativos conhecidos

1. **`GET /cdr/search` panica sempre** — `cdr_handler.go:20` usa `h.Config`, que `NewHandler` nunca atribui (`handler.go:21-27`). Sem middleware de recover.
2. **`LoadSnapshot` é código morto** (`queues/persist.go:140-155`) — o snapshot é write-only. **Estado de fila não sobrevive a restart**; `reconcile` reconstrói em até 60 s, mas métricas de 15 min e agregados do dia são perdidos a cada deploy.
3. **`s.version++` é incondicional** (`reducer.go:88`) e o fan-out manda todos os eventos ao dashboard — `answer`, `hangup`, `dtmf_*` bumpam a versão sem mudar nada, invalidando o ETag à toa e reescrevendo snapshot a cada 250 ms em pico.
4. **PII em log de produção**: `reducer.go:312` dumpa o evento inteiro; `resolver_handler.go:45` loga o corpo do resolver, com **nome e telefone de morador**.
5. **`PJSIP/` hardcoded** em `queue.go:15,24,31` — PBX com peer SIP legado falha silenciosamente.
6. **Proxy tipado é lossy**: `HandleDynamicResolver` faz unmarshal para struct e re-serializa. O Lua consome `res.config.prefix` e `res.config.dial_timeout`, que **não existem em `ResolverConfig`** — chegam sempre `nil`. Adicionar campo no resolver da API **exige recompilar o AMIgow**.

## Multi-instância: não funciona hoje

Porta 8080 hardcoded, `amigow.service` fixo, `/opt/amigow` fixo, `container_name` fixo, `location /amigow/` fixo. Segunda instância → `log.Fatalf` (`address already in use`) e crash loop de 5 s com `Restart=always`.

**`base_path` não resolve isso** — é metadado do OpenAPI. Rodar duas instâncias exige patch no código ou duplicação manual completa. Se o requisito de negócio é mesmo uma instância por PBX na mesma VPS, isso é uma lacuna de implementação, não de configuração.

## Observabilidade

`/health` é **estático** `{"status":"ok"}` — responde `ok` com o Asterisk desconectado, o MySQL fora e zero webhooks entregues. **Não use como sinal de saúde.** Os sinais reais (`ami.connected`, `ami.last_event_at`, `ami.reconnects`) estão dentro de `GET /queues/dashboard`, autenticado.

Sem Prometheus, sem níveis de log. Os 6 alertas de fila (`LONGEST_WAIT`, `WAITING`, `SLA_LOW`, `ABANDON_RATE`, `ASA`, `PAUSED_AGENTS`) **não notificam ninguém** — são campos JSON no dashboard, e `StartedAt` é recalculado a cada snapshot, então não há duração de alerta.

## Testes

Só `internal/ami/` tem testes — 20 funções, ~660 linhas, cobrindo bem os casos difíceis (frame partido, terminador dividido, ações concorrentes, reconexão). **Zero testes** em `webhook`, `api`, `queues`, `cdr`, `config`, `freepbx`, `cmd`.

A distribuição é o inverso do risco: o testado é a parte mais robusta; o não testado é onde estão todos os bugs acima. Ao mexer em `webhook/` ou `api/`, **considere abrir o primeiro teste do pacote**.

## Para o próximo agente

1. Leia `../engineering-context/systems/amigow.md` — tem os desconhecidos de maior consequência.
2. Antes de mexer no caminho do interfone, teste `curl` sem o header `X-API-Key`: não está confirmado se a lib Lua do PBX injeta a key.
3. Bug de webhook: confirme primeiro se o evento chegou a ser gerado (log `[FAN] webhook drop`) antes de investigar entrega.
4. Registre decisão consequente em `../engineering-context/decisions/` e rode `just validate`.
