# AMIgow - Asterisk AMI Connector

Go service to connect multiple Asterisk AMI servers, process specific events (Answer, Hangup, Missed Calls) and send them to webhooks, plus receive actions via REST API.

## Features

- Uses [goami](https://github.com/heltonmarx/goami) library for AMI communication
- Multiple simultaneous AMI connections with context support
- Smart event detection:
  - **Answer**: Answered calls
  - **Hangup**: Disconnected calls
  - **Missed**: Missed calls (unanswered/busy)
  - **Invite**: New calls (ring)
- Event sending to webhooks with configurable filters
- REST API for sending actions to Asterisk
- Automatic reconnection on failure
- Webhook retry (1 attempt)
- Configuration via external API with local fallback
- Graceful shutdown with context cancellation

## Project Structure

```
/cmd/main.go                  # Entry point with context
/internal/
  ├── ami/
  │   ├── manager.go          # AsteriskManager using goami
  │   ├── events.go           # AMI event processing
  │   └── actions.go          # Sending actions via goami
  ├── webhook/
  │   └── sender.go           # Webhook processing and sending
  ├── api/
  │   └── handler.go          # REST API handlers
  └── config/
      ├── config.go           # Configuration structures
      └── loader.go           # Config loader
/config.json                  # Local fallback configuration
```

## Configuration

Example `config.json`:

```json
{
  "ami_servers": [
    {
      "id": "asterisk-01",
      "host": "192.168.1.10",
      "port": 5038,
      "username": "admin",
      "password": "secret",
      "webhooks": [
        {
          "url": "https://api.example.com/events",
          "timeout_seconds": 10,
          "events_filter": ["answer", "hangup", "missed", "invite"]
        }
      ]
    }
  ],
  "config_api_url": "https://config.api.com/amigow/config",
  "config_refresh_minutes": 5
}
```

### Event Filters

- `answer`: Call was answered
- `hangup`: Call was disconnected
- `missed`: Call was not answered (NOANSWER, BUSY, CANCEL)
- `invite`: New call arriving (ringing)
- `*`: All events above

## Installation

```bash
# Install dependencies
go mod download

# Build
go build -o amigow cmd/main.go

# Run
./amigow
```

## Usage with Hot Reload (Development)

```bash
# Install Air
go install github.com/cosmtrek/air@latest

# Run with hot reload
air
```

## Docker

```bash
# Build
docker build -t amigow .

# Run
docker run -p 8080:8080 \
  -v $(pwd)/config.json:/root/config.json \
  -e CONFIG_API_URL=http://your-api:9000/config \
  amigow

# Docker Compose
docker-compose up -d
```

## REST API

### Send Action

```bash
curl -X POST http://localhost:8080/action \
  -H "X-Asterisk-ID: asterisk-01" \
  -H "Content-Type: application/json" \
  -d '{"Action":"Ping"}'
```

### Send CLI Command

```bash
curl -X POST http://localhost:8080/action \
  -H "X-Asterisk-ID: asterisk-01" \
  -H "Content-Type: application/json" \
  -d '{"Action":"Command","Command":"core show channels"}'
```

### Hangup Channel

```bash
curl -X POST http://localhost:8080/action \
  -H "X-Asterisk-ID: asterisk-01" \
  -H "Content-Type: application/json" \
  -d '{"Action":"Hangup","Channel":"SIP/1001-00000001","Cause":"16"}'
```

### Health Check

```bash
curl http://localhost:8080/health
```

**Response:**
```json
{
  "status": "ok",
  "managers": 1
}
```

## Webhook Payload

Example payload sent to webhooks:

```json
{
  "event_type": "answer",
  "source": "asterisk-01",
  "timestamp": "2026-01-21T10:30:00Z",
  "channel": "SIP/1001-00000001",
  "caller_id": "1001",
  "caller_name": "João Silva",
  "destination": "5000",
  "cause": "",
  "cause_text": "",
  "duration": "",
  "raw_data": {
    "Event": "Newchannel",
    "Channel": "SIP/1001-00000001",
    "ChannelState": "6",
    "ChannelStateDesc": "Up",
    "CallerIDNum": "1001",
    "CallerIDName": "João Silva",
    "Exten": "5000",
    "Context": "default",
    "Uniqueid": "1737456600.123"
  }
}
```

### Webhook Headers

- `Content-Type: application/json`
- `X-Event-Type: answer|hangup|missed|invite`
- `X-Source: asterisk-01`

## Environment Variables

- `CONFIG_API_URL`: Configuration API URL (default: http://localhost:9000/config)

## Event Detection

### Answer (Call Answered)

- `Newchannel` event with `ChannelStateDesc: Up`
- `Bridge` event with `BridgeState: Link`

### Hangup (Call Disconnected)

- `Hangup` event (any cause)

### Missed (Missed Call)

- `DialEnd` event with `DialStatus: NOANSWER|BUSY|CANCEL`
- `Hangup` event with `Cause: 17|18|19|21`
  - 17: USER_BUSY
  - 18: NO_USER_RESPONSE
  - 19: NO_ANSWER
  - 21: CALL_REJECTED

### Invite (New Call)

- `Newchannel` event with `ChannelStateDesc: Ring|Ringing`

## Logs

### Console Logs (stdout)

All application logs are written to stdout:

- `[CONFIG]` - Configuration loading
- `[AMI-ID]` - AMI connection and processing events
- `[WEBHOOK]` - Webhook sending
- `[API]` - REST API requests

Examples:
```
[asterisk-01] Connecting to AMI at 192.168.1.10:5038
[asterisk-01] Connected and authenticated successfully
[asterisk-01] Stream log will be saved to: ami_stream_asterisk-01.log
[asterisk-01] Starting event loop
[asterisk-01] Event answer processed: Channel=SIP/1001-00000001, CallerID=1001
[WEBHOOK] Webhook answer sent successfully to https://api.example.com/events
```

### AMI Stream Logs (Files)

Each AMI connection generates a log file with **ALL raw AMI stream** (sent and received):

- **File**: `ami_stream_<asterisk-id>.log`
- **Format**: Complete timeline with direction (SEND/RECV)
- **Content**: AMI stream clone without filters
- **Rotation**: Append mode (accumulates everything)

#### Log Format

The log is a timeline where:
- `<<< SEND` = Data sent TO Asterisk
- `>>> RECV` = Data received FROM Asterisk

Content example:
```
<<< SEND [2026-01-22 15:30:40.100]
Action: Login
Username: amigow
Secret: ********
Events: system,call,all
ActionID: 1234-5678-9012

>>> RECV [2026-01-22 15:30:40.105]
Response: Success
Message: Authentication accepted

>>> RECV [2026-01-22 15:30:45.123]
Event: Newchannel
Privilege: call,all
Channel: SIP/1001-00000001
ChannelState: 0
ChannelStateDesc: Down
CallerIDNum: 1001
CallerIDName: João Silva
Uniqueid: 1737556245.123
Context: default
Exten: 5000
Priority: 1

<<< SEND [2026-01-22 15:30:50.200]
Action: Hangup
Channel: SIP/1001-00000001
Cause: 16
ActionID: 1234-5678-9013

>>> RECV [2026-01-22 15:30:50.205]
Response: Success
Message: Channel SIP/1001-00000001 hungup

>>> RECV [2026-01-22 15:30:50.210]
Event: Hangup
Privilege: call,all
Channel: SIP/1001-00000001
Cause: 16
Cause-txt: Normal Clearing
Duration: 45
```

**Important**: 
- `.log` files are automatically added to `.gitignore`
- Captures 100% of AMI stream without filters
- Allows complete debugging of Asterisk communication

## Error Handling

### AMI Connection Fails

- Attempts to reconnect after 2 seconds
- If fails: terminates goroutine and logs error
- Other AMIs continue working

### Webhook Fails

- Log with details (URL, event, status)
- Waits 1 second
- Attempts 1 retry
- If fails: discards event and continues

### Action Fails

- Returns HTTP 500 with JSON error
- Does not close AMI connection
- Client can try again

## Manual Tests

### 1. Test AMI Connection

```bash
# Via telnet
telnet 192.168.1.10 5038

# Via nc (netcat)
nc -zv 192.168.1.10 5038
```

### 2. Test Events

- Make a call and answer → `answer` event
- Hangup active call → `hangup` event
- Call and don't answer → `missed` event
- Start call → `invite` event

### 3. Test Webhooks

Check arrival at destination with:
```bash
# Example with ngrok or webhook.site
curl -X POST https://webhook.site/your-id -d '{"test": true}'
```

### 4. Test API

```bash
# Ping
curl -X POST http://localhost:8080/action \
  -H "X-Asterisk-ID: asterisk-01" \
  -H "Content-Type: application/json" \
  -d '{"Action":"Ping"}'

# Core Status
curl -X POST http://localhost:8080/action \
  -H "X-Asterisk-ID: asterisk-01" \
  -H "Content-Type: application/json" \
  -d '{"Action":"CoreStatus"}'
```

## Dependencies

- Go 1.21+
- [goami](https://github.com/heltonmarx/goami) v1.0.0

## Principles

This project follows simplicity principles:
- Explicit and readable code
- Go stdlib + goami
- No magic abstractions
- Context for cancellation
- Simple logs with log.Println
- Explicit error handling

## Queues Dashboard

O AMIgow mantém um **snapshot vivo** das filas em memória, alimentado pelos eventos AMI e enriquecido com metadados do FreePBX (banco `asterisk`). O endpoint nunca consulta AMI/DB sob demanda.

### Habilitar

No `config.json`:

```json
"dashboard": {
  "enabled": true,
  "service_level_target_seconds": 20,
  "reconcile_seconds": 60,
  "persist_snapshot_seconds": 5,
  "recommended_polling_ms": 2000,
  "min_polling_ms": 1000,
  "snapshot_path": "/var/lib/amigow/queue-dashboard.snapshot.json",
  "events_path": "/var/lib/amigow/queue-dashboard.events.ndjson"
}
```

O dashboard reusa a conexão `cdr_db` para ler `queues_config` e `queues_details` do FreePBX, então não exige nova credencial.

### Endpoint

```
GET /queues/dashboard
GET /queues/dashboard?since=<version>
```

Headers suportados:

- `If-None-Match: "queues-dashboard-<version>"` → retorna 304 quando não mudou.

Resposta resumida:

```json
{
  "version": 18492,
  "generated_at": "2026-06-02T15:10:22-03:00",
  "polling": { "recommended_interval_ms": 2000, "min_interval_ms": 1000 },
  "ami":     { "connected": true, "last_event_at": "...", "last_snapshot_at": "...", "reconnects": 0 },
  "totals":  { "queues": 3, "waiting": 7, "longest_wait_seconds": 74, "agents_total": 16, "agents_available": 5, "agents_in_call": 8, "offered_15m": 58, "answered_15m": 51, "abandoned_15m": 3, "service_level_15m": 78.4 },
  "queues":  [ ... ],
  "alerts":  [ ... ]
}
```

### Como funciona

- Boot: carrega metadados (`queues_config`/`queues_details`), executa `Action: QueueStatus` no AMI e monta snapshot inicial.
- Cada evento AMI relevante (`QueueCallerJoin`, `QueueCallerAbandon`, `AgentConnect`, `AgentComplete`, `QueueMemberStatus`, etc.) atualiza o snapshot e bumpa a `version`.
- Métricas: janela rolling de 15 minutos + acumulador `today` (offered/answered/abandoned/SLA/ASA/talk).
- Reconciliação: a cada 60s reaplica `QueueStatus` para autocorrigir divergências.
- Persistência: `snapshot.json` reescrito via `rename` atômico a cada 5s **OU** 250ms após qualquer evento AMI aplicado (debounce, o que vier primeiro). Eventos importantes em `events.YYYY-MM-DD.ndjson` (append diário com retenção configurável).

### Fluxo realtime e auditoria

O caminho de cada evento é:

```
AMI -> eventChan -> fanOutEvents -> dashboardChan -> RunReducer -> SnapshotStore (mem) -> endpoint
                                                                              `-> dirty -> snapshot.json (debounced 250ms)
```

Para auditar em runtime, três classes de log permitem rastrear o pipeline ponta-a-ponta:

- `[AMI] Evento: ...` — evento bruto recebido do Asterisk.
- `[REDUCER] <tipo> queue=<id> version=<n>` — confirma que o reducer aplicou e bumpou a `version` do snapshot.
- `[DASH] GET dashboard since=X current=Y resp=200|304` — exatamente o que o endpoint devolveu.
- `[FAN] dashboard drop (total=N)` — só aparece se o canal do dashboard encher (não deveria ocorrer em uso normal, buffer 1000).

Se um evento aparece em `[AMI]` mas não em `[REDUCER]`, ele foi descartado pelo `ProcessAMIEvent` (tipo não monitorado) ou pelo fan-out (canal cheio).

### Polling no frontend

Recomendado polling de 2s no JSON; o contador visual de espera deve ser local no frontend (use `entered_at`).

```ts
const waitSeconds = Math.floor((Date.now() - new Date(caller.entered_at).getTime()) / 1000);
```

## Troubleshooting

### "socket error"

- Check if Asterisk is accessible
- Check firewall/ports
- Test with `nc -zv host port`

### "login error"

- Check AMI credentials
- Check `/etc/asterisk/manager.conf`
- Check permissions (read/write)

### Events don't arrive

- Check if `events_filter` is correct
- Check logs to see processed events
- Temporarily test with `events_filter: ["*"]`

### Webhook doesn't receive

- Check webhook URL
- Check timeout
- Check retry logs
- Test webhook manually with curl

### nginx
location /amigow/ {
        # MUDE AQUI: adicione /amigow/ no final
        proxy_pass http://127.0.0.1:8080/amigow/;

        # Headers obrigatórios
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Evita problemas com streaming / long polling
        proxy_http_version 1.1;
        proxy_set_header Connection "";

        # Timeouts (importante pra AMI/webhook)
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;

        # 1) Remove CSP que venha do serviço em 8080 (senão fica duplicado e bloqueia)
        proxy_hide_header Content-Security-Policy;
        proxy_hide_header Content-Security-Policy-Report-Only;

        # 2) Define um CSP único (TUDO EM UMA LINHA)
        add_header Content-Security-Policy "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self';" always;
}
## License

MIT License


## Setup
go build -o amigow cmd/main.go

### copiar config do db para o config.json
ssh -t safehouse_freepbx_hx "sudo cat /etc/freepbx.conf"

### criar user AMI

### enviar .sh de setup
scp ./setup-amigow.sh safehouse_freepbx_hx:amigow/setup-amigow.sh
scp ./amigow safehouse_freepbx_hx:amigow/amigow
scp ./config.json safehouse_freepbx_hx:amigow/config.json
 

### updates
scp ./amigow safehouse_freepbx_hx:amigow/amigow

ssh safehouse_freepbx_hx
sudo systemctl stop amigow
sudo cp ./amigow/amigow /opt/amigow/amigow
sudo cp ./amigow/config.json /opt/amigow/config.json
sudo systemctl start amigow
sudo journalctl -u amigow -f

az account set --subscription 7388d489-5e8e-4f85-9967-93b1e14f951b
az vm run-command invoke \
  -g monolito-production_group \
  -n freepbx-hx \
  --command-id RunShellScript \
  --scripts '
sudo fwconsole firewall trust 177.107.232.238/32 || true
sudo fwconsole firewall restart || true'

az vm run-command invoke \
  -g monolito-production_group \
  -n freepbx-hx \
  --command-id RunShellScript \
  --scripts '/home/azureuser/tip.sh 177.107.232.238'

az vm run-command invoke \
  -g monolito-production_group \
  -n freepbx-hx \
  --command-id RunShellScript \
  --scripts 'sudo fwconsole firewall f2b2'


  az vm run-command invoke \
  -g monolito-production_group \
  -n freepbx-hx \
  --command-id RunShellScript \
  --scripts '
IP="177.107.232.238"

sudo fwconsole firewall trust "$IP/32"
sudo fwconsole firewall sync
sudo fwconsole firewall restart 
"
for jail in $(sudo fail2ban-client status | sed -n "s/.*Jail list:\s*//p" | tr "," " "); do
  echo "Unban $IP from $jail"
  sudo fail2ban-client set "$jail" unbanip "$IP" || true
done

sudo fwconsole firewall del blacklist "$IP/32" || true
sudo fwconsole firewall restart
sudo fwconsole firewall f2bs
'

az vm run-command invoke \
  -g monolito-production_group \
  -n freepbx-hx \
  --command-id RunShellScript \
  --scripts '
IP="45.176.64.186"


for jail in $(sudo fail2ban-client status | sed -n "s/.*Jail list:\s*//p" | tr "," " "); do
  echo "Unban $IP from $jail"
  sudo fail2ban-client set "$jail" unbanip "$IP" || true
done

sudo fwconsole firewall del blacklist "$IP/32" || true
sudo fwconsole firewall restart
sudo fwconsole firewall f2bs
'

for jail in $(sudo fail2ban-client status | sed -n "s/.*Jail list:\s*//p" | tr "," " "); do
  echo "Unban $IP from $jail"
  sudo fail2ban-client get "$jail" status "$IP" || true
done