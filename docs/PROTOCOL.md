# Protocolo VAIJUNTO (API remota sobre TCP)

Um cliente em **qualquer linguagem** deve conseguir falar com o servidor lendo só este arquivo. Há um exemplo em `examples/python_ping.py`.

Versão do protocolo: **1**.

## 1. Transporte

- TCP/IP. Servidor: `net.Listen("tcp", "0.0.0.0:5000")` (porta configurável).
- Cliente: `connect(SERVER_HOST, SERVER_PORT)`.
- Uma conexão = uma sessão. Várias requisições na mesma conexão, em sequência (request/response). Não há pipelining nesta versão: o cliente espera a resposta antes do próximo pedido.
- Não é HTTP, não é WebSocket, não é gRPC.

## 2. Framing (encapsulamento)

TCP é um **fluxo de bytes**. `Write` não define mensagem. O frame é:

```text
offset 0..3  uint32 big-endian = N   (tamanho do payload, 1 ≤ N ≤ MAX_PAYLOAD)
offset 4..4+N-1  payload UTF-8 JSON
```

`MAX_PAYLOAD` padrão: **1048576** (1 MiB), variável `MAX_PAYLOAD`.

Regras:

1. Ler exatamente 4 bytes (`struct.unpack(">I", header)` em Python).
2. Se `N == 0` ou `N > MAX_PAYLOAD`, rejeitar e **fechar** a conexão (`INVALID_FRAME`).
3. Ler exatamente N bytes (loop até completar; `recv` pode devolver menos).
4. Interpretar o payload como JSON UTF-8.

Write parcial: o emissor deve repetir `send` até enviar header+payload inteiros.

Duas mensagens coalescidas no mesmo `recv` são normais: o receptor sempre consome 4+N e deixa o resto no buffer do TCP.

## 3. Envelope

### Request

```json
{
  "version": 1,
  "operation": "SEARCH_ITINERARIES",
  "requestId": "req-001",
  "data": {}
}
```

| Campo | Tipo | Obrigatório | Notas |
|---|---|---|---|
| version | number | sim | deve ser `1` |
| operation | string | sim | ver catálogo |
| requestId | string | sim | correlator; também chave de idempotência em `CONFIRM_RESERVATION` |
| data | object | não | payload da operação; `{}` se vazio |

### Response

```json
{
  "version": 1,
  "requestId": "req-001",
  "status": "OK",
  "data": {},
  "error": null
}
```

Erro:

```json
{
  "version": 1,
  "requestId": "req-001",
  "status": "ERROR",
  "data": null,
  "error": { "code": "NO_SEATS", "message": "one or more segments have no seats" }
}
```

`requestId` é ecoado quando o JSON pôde ser lido. JSON inválido pode devolver `requestId` vazio.

## 4. Códigos de erro

| code | quando |
|---|---|
| INVALID_FRAME | tamanho 0, absurdo, ou frame incompleto (conexão tende a fechar) |
| INVALID_JSON | payload não é JSON objeto |
| VALIDATION_ERROR | version≠1, campos faltando, rota inválida, legs sobrepostas/desconectadas/datas incompatíveis |
| UNKNOWN_OPERATION | `operation` desconhecida |
| UNAUTHENTICATED | operação protegida sem `LOGIN` |
| FORBIDDEN | papel errado ou credencial inválida |
| NOT_FOUND | carona/reserva inexistente |
| NO_SEATS | revalidação falhou; nada foi reservado |
| RESERVATION_CONFLICT | carona inativa etc. |
| ALREADY_CANCELLED | reservado para simetria; cancelamento idempotente devolve OK |
| INTERNAL_ERROR | falha inesperada |

Mensagem inválida **não altera** o estado compartilhado (INV-9).

## 5. Fluxo de conexão

```text
TCP connect
  → PING (opcional, sem login)
  → LOGIN  {username, password}
  → operações do papel
  → LOGOUT (servidor responde e fecha)  OU  TCP close
```

Queda abrupta: a goroutine da conexão termina; reservas já confirmadas permanecem; nada fica “preso” porque o lock nunca é segurado durante I/O.

Após **reinício do servidor**, a conexão cai. O cliente conecta de novo e faz `LOGIN` com a mesma conta (persistida). `LIST_RESERVATIONS` (e `SEARCH_ITINERARIES`) vêm do JSON no `DATA_PATH` do processo; o cache da CLI (`lastReservations`) começa vazio. Trocar o IP do host não recria o arquivo — desde que o servidor abra o **mesmo** `DATA_PATH` (absoluto ou volume Docker `/data/state.json`).

## 6. Autenticação e papéis

Contas seed (senha `senha123`):

- `motorista1`, `motorista2` → `DRIVER`
- `passageiro1`, `passageiro2`, `passageiro3` → `PASSENGER`

`REGISTER` cria conta nova persistida.

| operation | autenticação | papel |
|---|---|---|
| PING | não | — |
| REGISTER | não | — |
| LOGIN | não | — |
| LOGOUT | irrelevante | — |
| PUBLISH_RIDE | sim | DRIVER |
| LIST_DRIVER_RIDES | sim | DRIVER |
| CANCEL_RIDE | sim | DRIVER |
| LIST_RIDE_PASSENGERS | sim | DRIVER |
| SEARCH_ITINERARIES | sim | PASSENGER |
| CONFIRM_RESERVATION | sim | PASSENGER |
| LIST_RESERVATIONS | sim | PASSENGER |
| CANCEL_RESERVATION | sim | PASSENGER |

## 7. Operações e campos

Preços: **centavos** (`int64`). Datas: `YYYY-MM-DD`. Horário: `HH:MM` 24h.

O **fio não mudou** nesta revisão de usabilidade. Só as CLIs humanas (`cmd/driver`, `cmd/passenger`) convertem data `DD/MM/AAAA` e preço em reais (`15` / `15,50` / `15.50` → centavos) na borda. `state.json`, testes de protocolo e um cliente Python continuam falando ISO + centavos.

### PING

`data`: `{}` → `{ "message": "PONG" }`

### LOGIN

```json
{ "username": "passageiro1", "password": "senha123" }
```

OK: `{ "userId", "username", "role" }`

### REGISTER

```json
{ "username": "ana", "password": "segredo", "role": "PASSENGER" }
```

`role`: `DRIVER` ou `PASSENGER`.

### PUBLISH_RIDE

```json
{
  "cities": ["Salvador", "Feira de Santana", "Jequié"],
  "departureDate": "2026-10-10",
  "departureTime": "08:00",
  "capacity": 2,
  "segmentPrices": [1500, 2000]
}
```

`segmentPrices.length` deve ser `cities.length - 1`. Cidades distintas. Gera trechos adjacentes com `availableSeats = capacity`.

### LIST_DRIVER_RIDES

`data`: `{}` → `{ "rides": [ RideView, ... ] }`

RideView:

```json
{
  "rideId": "ride-…",
  "driverId": "user-…",
  "cities": ["Salvador", "Feira de Santana"],
  "departureDate": "2026-10-10",
  "departureTime": "08:00",
  "capacity": 2,
  "status": "ACTIVE",
  "segments": [
    { "segmentIndex": 0, "origin": "Salvador", "destination": "Feira de Santana",
      "price": 1500, "availableSeats": 2 }
  ]
}
```

`status`: `ACTIVE` | `CANCELLED`.

### CANCEL_RIDE

`{ "rideId": "ride-…" }` → RideView com `status=CANCELLED`. Idempotente se já cancelada. Cancela reservas confirmadas que usam essa carona.

### LIST_RIDE_PASSENGERS

`{ "rideId": "ride-…" }` → passageiros **confirmados** agrupados por trecho.

### SEARCH_ITINERARIES

```json
{ "origin": "Salvador", "destination": "Jequié", "date": "2026-10-10" }
```

A busca **não reserva**. Snapshot sob `RLock`.

```json
{
  "itineraries": [
    {
      "totalPrice": 3500,
      "transfers": 0,
      "legs": [
        {
          "rideId": "ride-…",
          "driverId": "user-…",
          "origin": "Salvador",
          "destination": "Jequié",
          "departureDate": "2026-10-10",
          "departureTime": "08:00",
          "price": 3500,
          "availableSeats": 2
        }
      ]
    }
  ]
}
```

`transfers` = número de trocas de motorista (`legs.length - 1` para o caso típico). Limite técnico: no máximo 3 baldeações e 20 resultados. Ordenação: preço total, depois menos transfers, depois IDs.

Não há filtro por horário de conexão (DEC-009).

### CONFIRM_RESERVATION

O cliente reenvia as `legs` (não há hold). O servidor **revalida** todos os trechos sob `Lock` e aplica tudo-ou-nada.

```json
{
  "legs": [
    { "rideId": "ride-aaa", "origin": "Salvador", "destination": "Feira de Santana" },
    { "rideId": "ride-bbb", "origin": "Feira de Santana", "destination": "Jequié" }
  ]
}
```

Cada leg é um intervalo contínuo **numa** carona (pode cobrir vários segmentos internos). O servidor expande cada leg nos segmentos físicos `(rideId, segmentIndex)` **antes** de mutar qualquer coisa.

Regras de validação do pedido (tudo `VALIDATION_ERROR`, estado intacto):

- cada `origin`/`destination` deve existir nessa carona, na ordem da rota;
- legs consecutivas devem conectar: destino da anterior = origem da seguinte;
- todas as caronas do pedido devem ter a **mesma** `departureDate` (regra same-day da busca; não há filtro por horário/ETA);
- o mesmo segmento físico não pode aparecer duas vezes no mesmo pedido (leg idêntica repetida **ou** sobreposição parcial, ex. A→C e B→C na rota A-B-C). O servidor **não** deduplica em silêncio.

Idempotência: o mesmo `(usuário da sessão, requestId)` devolve a mesma reserva sem consumir outro assento.

Erro `NO_SEATS`: nenhum trecho foi decrementado. Erro `VALIDATION_ERROR` nestes casos também não cria reserva, não altera `availableSeats` e não grava o arquivo.

### LIST_RESERVATIONS

`{}` → `{ "reservations": [ ReservationView ] }`

`status`: `CONFIRMED` | `CANCELLED`.

### CANCEL_RESERVATION

`{ "reservationId": "res-…" }` → ReservationView. Idempotente.

### LOGOUT

`{}` → `{ "message": "bye" }` e a conexão é fechada.

## 8. Exemplos completos (hex omitido; só o JSON do payload)

### LOGIN

Request:

```json
{"version":1,"operation":"LOGIN","requestId":"req-login","data":{"username":"motorista1","password":"senha123"}}
```

Response:

```json
{"version":1,"requestId":"req-login","status":"OK","data":{"userId":"user-driver-1","username":"motorista1","role":"DRIVER"},"error":null}
```

### SEARCH_ITINERARIES

Request:

```json
{"version":1,"operation":"SEARCH_ITINERARIES","requestId":"req-s","data":{"origin":"Salvador","destination":"Jequié","date":"2026-10-10"}}
```

### CONFIRM_RESERVATION (sucesso vs NO_SEATS)

Dois passageiros com 1 vaga: o primeiro recebe `status=OK`; o segundo:

```json
{"version":1,"requestId":"req-b","status":"ERROR","data":null,"error":{"code":"NO_SEATS","message":"one or more segments have no seats"}}
```

## 9. Sincronização cliente-servidor

- Request/response síncrono na conexão.
- Sem mensagens espontâneas do servidor (sem push).
- Relógio: o servidor não sincroniza relógio com o cliente; a data da busca é a informada no JSON.
- Quem “confirma primeiro”: vence quem entra primeiro na seção crítica do `Lock` (não o clique local).

## 10. Como um cliente Python envia um frame

```python
import struct, json
sock.sendall(struct.pack(">I", len(payload)) + payload)
```

Ver `examples/python_ping.py`.
