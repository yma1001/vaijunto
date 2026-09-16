# Testes

## Comandos

```bash
go test ./...
go test -race ./...
go vet ./...
bash scripts/smoke.sh
```

O race detector encontra data races, não deadlock lógico nem double-booking. Por isso existem testes de invariante e de disputa.

## Mapa de casos

| Caso | Onde |
|---|---|
| Round-trip, mensagens coalescidas, read de 1 byte | `internal/protocol/frame_test.go` |
| Header/payload incompletos, frame grande demais | idem + `TestIncompleteFrameAndAbruptDisconnect` |
| JSON inválido, op desconhecida | `internal/server/server_test.go` |
| Itinerário direto e composto, data, vaga 0 | `internal/search/graph_test.go` |
| Último assento (32 goroutines) | `TestLastSeatContention` |
| Último trecho indisponível → sem reserva parcial | `TestAtomicLastSegmentUnavailable` |
| CONFIRM simultâneo via TCP real | `TestSimultaneousConfirmTCP` |
| Cancelar duas vezes | `TestCancelIdempotent` |
| Mesmo requestId duas vezes | `TestConfirmIdempotentRequestID` / `TestRepeatedConfirmSameRequestIDOverTCP` |
| Disconnect no meio do frame | `TestIncompleteFrameAndAbruptDisconnect` |
| Muitos clientes PING | `TestManyClients` |
| Persistência + reinício do processo | `TestPersistenceRestartKeepsRides`, `TestPersistenceAcrossServerRestart` |
| Invariantes ≥0 e ≤capacity | `TestNeverNegativeOrAboveCapacity` |
| Carga / latência | `TestLoadishLatencyReported`, `cmd/loadtest` |

## Loadtest

Com o servidor no ar:

```bash
SERVER_HOST=127.0.0.1 SERVER_PORT=5000 LOADTEST_CLIENTS=20 go run ./cmd/loadtest
```

Publica uma carona com 1 assento e dispara N clientes TCP. Sucesso esperado: **1**. O JSON de saída traz `avg_ms`, `p95_ms`, `throughput_ops_s`. Não há meta oficial de latência — só medimos.

## Docker

```bash
docker build -t vaijunto-server --build-arg BUILD_TARGET=server .
docker build -t vaijunto-driver --build-arg BUILD_TARGET=driver .
docker build -t vaijunto-passenger --build-arg BUILD_TARGET=passenger .
docker build -t vaijunto-smoke --build-arg BUILD_TARGET=smoke .
docker run -d --name vj-server -p 5000:5000 -v vaijunto-data:/data vaijunto-server
docker run --rm -e SERVER_HOST=host.docker.internal -e SERVER_PORT=5000 vaijunto-smoke
# Linux: use o IP do host (ip -4 addr) em vez de host.docker.internal, ou
# --network host no smoke apontando para 127.0.0.1.
```

Reinício do container:

```bash
docker restart vj-server
# contas e caronas continuam no volume; LOGIN precisa ser refeito
```

## Demonstração no laboratório (PCs distintos)

Ver README seção Docker multi-máquina. O teste físico de LAN é pendência externa se o agente não tiver o laboratório; os scripts estão prontos.
