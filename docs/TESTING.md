# Testes

Quem clona [https://github.com/yma1001/vaijunto](https://github.com/yma1001/vaijunto) precisa de **Go 1.22+** (`go.mod` declara `go 1.22`; `go version`), **git** e, se quiser o caminho em container, **Docker**. Instalação do Go: [https://go.dev/dl/](https://go.dev/dl/). Contas da demo (senha `senha123`): ver README.

Linux é o caminho principal (laboratório da UEFS e a máquina do aluno). macOS e Windows usam os mesmos testes; mudam instalação, variáveis e o nome dos binários. A demo manual pede **três terminais** (ou três janelas do PowerShell).

## Comandos (Linux)

```bash
git clone https://github.com/yma1001/vaijunto.git
cd vaijunto
go test ./...
go test -race ./...
go vet ./...
go build -o bin/server ./cmd/server
go build -o bin/driver ./cmd/driver
go build -o bin/passenger ./cmd/passenger
bash scripts/smoke.sh
bash scripts/docker-local.sh   # servidor em container + smoke + restart
```

O race detector encontra data races, não deadlock lógico nem double-booking. Por isso existem testes de invariante e de disputa.

Três terminais, senha `senha123`:

```bash
# terminal 1 — na raiz do clone; absoluto evita segundo state.json se o cwd mudar
export DATA_PATH="$PWD/data/state.json"
./bin/server

# terminal 2
export SERVER_HOST=127.0.0.1
export SERVER_PORT=5000
./bin/driver

# terminal 3
export SERVER_HOST=127.0.0.1
export SERVER_PORT=5000
./bin/passenger
```

## macOS

Go: `brew install go` ou o instalador de [go.dev/dl](https://go.dev/dl/). `go version` ≥ 1.22.

Os comandos Linux acima valem iguais (`export SERVER_HOST`, `SERVER_PORT`, `DATA_PATH`; `./bin/server` etc.). Smoke: `bash scripts/smoke.sh`. Docker: Docker Desktop, depois `bash scripts/docker-local.sh` ou os `docker` da seção Docker.

## Windows

Go: msi em [go.dev/dl](https://go.dev/dl/). Git for Windows traz **Git Bash**, que segue os comandos Linux.

PowerShell (três janelas; `$env:SERVER_HOST`, `$env:SERVER_PORT`, `$env:DATA_PATH`; binários `bin\*.exe`):

```powershell
git clone https://github.com/yma1001/vaijunto.git
cd vaijunto
go test ./...
go test -race ./...
go vet ./...
go build -o bin/server.exe ./cmd/server
go build -o bin/driver.exe ./cmd/driver
go build -o bin/passenger.exe ./cmd/passenger
```

```powershell
# janela 1
$env:DATA_PATH = (Join-Path (Get-Location) "data\state.json")
.\bin\server.exe

# janela 2
$env:SERVER_HOST = "127.0.0.1"
$env:SERVER_PORT = "5000"
.\bin\driver.exe

# janela 3
$env:SERVER_HOST = "127.0.0.1"
$env:SERVER_PORT = "5000"
.\bin\passenger.exe
```

Se `go test -race` falhar por CGO/gcc, instale MinGW-w64 ou use Git Bash/WSL; `go test ./...` cobre a suíte sem o detector.

Smoke: `bash scripts/smoke.sh` no Git Bash. No PowerShell nativo, `go test ./...` + os três `.exe`.

Firewall: localhost costuma funcionar. Para outro PC, libere TCP 5000 no Windows Defender Firewall se a conexão for recusada.

Docker: Docker Desktop. `docker build` / `docker compose` no PowerShell; `bash scripts/docker-local.sh` no Git Bash.

## Mapa de casos

| Caso | Onde |
|---|---|
| Round-trip, mensagens coalescidas, read de 1 byte | `internal/protocol/frame_test.go` |
| Header/payload incompletos, frame grande demais | idem + `TestIncompleteFrameAndAbruptDisconnect` |
| JSON inválido, op desconhecida | `internal/server/server_test.go` |
| JSON inválido no CONFIRM via TCP (estado intacto) | `TestInvalidConfirmJSONOverTCPDoesNotMutate` |
| Itinerário direto e composto, data, vaga 0 | `internal/search/graph_test.go` |
| Último assento (32 goroutines) | `TestLastSeatContention` |
| Último trecho indisponível → sem reserva parcial | `TestAtomicLastSegmentUnavailable` |
| CONFIRM simultâneo via TCP real | `TestSimultaneousConfirmTCP` |
| Cancelar duas vezes | `TestCancelIdempotent` |
| Mesmo requestId duas vezes | `TestConfirmIdempotentRequestID` / `TestRepeatedConfirmSameRequestIDOverTCP` |
| Leg idêntica repetida, capacidade 1 (não vai a −1) | `TestConfirmRejectsDuplicateIdenticalLegCapacity1` |
| Leg idêntica repetida, capacidade > 1 | `TestConfirmRejectsDuplicateIdenticalLegCapacityGreaterThan1` |
| Sobreposição parcial de segmentos (A→C e B→C) | `TestConfirmRejectsPartialSegmentOverlap` |
| Rejeição não muta assentos/reservas/arquivo | `TestConfirmRejectLeavesSeatsReservationsAndFileUnchanged` |
| Itinerário desconectado / datas diferentes | `TestConfirmRejectsDisconnectedItinerary`, `TestConfirmRejectsIncompatibleDates` |
| Composto válido de motoristas distintos | `TestConfirmValidCompositeDistinctDrivers` |
| Dois legs consecutivos da mesma carona | `TestConfirmValidConsecutiveSegmentsSameRide` |
| Leg duplicada via TCP real → VALIDATION_ERROR | `TestConfirmDuplicateLegsRejectedOverTCP` |
| Disconnect no meio do frame | `TestIncompleteFrameAndAbruptDisconnect` |
| Muitos clientes PING | `TestManyClients` |
| Persistência + reinício do processo | `TestPersistenceRestartKeepsRides`, `TestPersistenceAcrossServerRestart` |
| Reserva + LIST após novo `Store` / restart TCP | `TestConfirmReservationSurvivesNewStoreLoad`, `TestListReservationsAfterServerRestart` |
| `confirmIndex` reconstruído no load | `TestLoadRebuildsConfirmIndexFromReservations` |
| Invariantes ≥0 e ≤capacity | `TestNeverNegativeOrAboveCapacity` |
| Carga / latência | `TestLoadishLatencyReported`, `cmd/loadtest` |
| Busca não reserva (INV-5) | `TestSearchDoesNotReserve` |
| Itinerário composto via TCP | `TestCompositeSearchAndConfirmTCP` |
| REGISTER + LOGOUT fecha a conexão | `TestRegisterLoginLogout` |
| CLI logout volta ao menu, troca de conta, só 0 sai | `cmd/passenger/main_test.go`, `cmd/driver/main_test.go` |
| REGISTER DRIVER+PASSENGER, duplicata, persistência após `Store` restart | `TestRegisterBothRolesDuplicateAndRestart`, `TestRegisterBothRolesDuplicateAndRestartTCP` |
| Rota com 2 e 4+ cidades | `TestPublishRideTwoAndFourCities`, `TestParseCitiesTwoAndFour` |
| Data BR na CLI (`DD/MM/AAAA` ↔ ISO) | `internal/cliui/cliui_test.go` (`TestParseBRDate`) |
| Preço BRL → centavos (vírgula, ponto, inteiro, inválido) | `TestParseBRLToCents`, `TestParseBRLToCentsInvalid` |
| Listagem formatada (sem JSON cru) | `TestFormatRideAndItineraryNoRawJSON` |
| Motorista cancela carona (passageiro vê CANCELLED) | `TestDriverCancelRideTCP` |
| Validação do envelope | `internal/protocol/message_test.go` |

A CLI não imprime JSON cru: caronas, buscas e reservas saem com rótulos, data `DD/MM/AAAA`, rota com `→`, status em português (`ATIVA` / `CANCELADA` / `CONFIRMADA`) e `R$ 15,00`. O protocolo e o arquivo persistido continuam ISO + centavos.

`DATA_PATH` relativo vale em relação ao **cwd do processo**, não ao IP e não ao diretório do binário. `./bin/server` e Docker (`/data/state.json` no volume `vaijunto-data`) são arquivos diferentes; ambos nascem com o seed, então o LOGIN funciona e as reservas “somem”. Prefira `$PWD/data/state.json` no Ubuntu e o volume nomeado no Docker. Testes usam `t.TempDir()` — nunca o `data/state.json` do aluno.

Demo manual isolada (não use o `data/state.json` do aluno; `DATA_PATH` temporário + porta livre):

```bash
TMP=$(mktemp -d)
export DATA_PATH="$TMP/state.json"
export SERVER_HOST=127.0.0.1
export SERVER_PORT=15001
./bin/server &
# em outros terminais: ./bin/driver e ./bin/passenger
# criar conta motorista e passageiro; publicar 4 cidades; data BR; preços R$;
# buscar; reservar; listar formatado; cancelar; matar o servidor; subir de novo;
# LOGIN nas mesmas contas e conferir persistência.
```

Amostra real da CLI (DATA_PATH temporário; protocolo no fio continuou ISO + centavos):

```text
1) entrar  2) criar conta  3) PING  0) sair
> 2
usuário: motorista_cli
...
Conta criada.
  ID: user-2ee3277d0e59357a
Você já pode entrar com esse usuário.

Cidades da rota, separadas por vírgula (mínimo 2)
exemplo: Salvador, Feira de Santana, Jequié, Vitória da Conquista
> Salvador, Feira de Santana, Jequié, Vitória da Conquista
data (DD/MM/AAAA): 10/10/2026
Preço Salvador → Feira de Santana (R$): 15
Preço Feira de Santana → Jequié (R$): 20,50
Preço Jequié → Vitória da Conquista (R$): 25.00
Carona
  ID: ride-32753ead6c73854c
  Status: ATIVA
  Rota: Salvador → Feira de Santana → Jequié → Vitória da Conquista
  Data: 10/10/2026
  Trechos:
    1. Salvador → Feira de Santana
       Preço: R$ 15,00
```

## Loadtest

Com o servidor no ar (Linux/macOS):

```bash
export SERVER_HOST=127.0.0.1
export SERVER_PORT=5000
export LOADTEST_CLIENTS=20
go run ./cmd/loadtest
```

PowerShell:

```powershell
$env:SERVER_HOST = "127.0.0.1"
$env:SERVER_PORT = "5000"
$env:LOADTEST_CLIENTS = "20"
go run ./cmd/loadtest
```

Publica uma carona com 1 assento e dispara N clientes TCP. Sucesso esperado: **1**. O JSON de saída traz `avg_ms`, `p95_ms`, `throughput_ops_s`. Não há meta oficial de latência — só medimos.

## Docker

Linux: Docker Engine. macOS e Windows: Docker Desktop (os `docker build` / `docker run` abaixo valem; `bash scripts/docker-local.sh` no Git Bash no Windows).

```bash
docker build -t vaijunto-server --build-arg BUILD_TARGET=server .
docker build -t vaijunto-driver --build-arg BUILD_TARGET=driver .
docker build -t vaijunto-passenger --build-arg BUILD_TARGET=passenger .
docker build -t vaijunto-smoke --build-arg BUILD_TARGET=smoke .
docker run -d --name vj-server -p 5000:5000 -v vaijunto-data:/data vaijunto-server
docker run --rm -e SERVER_HOST=host.docker.internal -e SERVER_PORT=5000 vaijunto-smoke
# Docker Desktop (macOS/Windows): host.docker.internal aponta para o host.
# Linux: use o IP do host (ip -4 addr) em vez de host.docker.internal, ou
# --network host no smoke apontando para 127.0.0.1.
```

Reinício do container:

```bash
docker restart vj-server
# contas e caronas continuam no volume; LOGIN precisa ser refeito
```

## Demonstração no laboratório (PCs distintos)

Orientada a **Linux** (três PCs da UEFS). Passo a passo completo (Compose, IP da LAN, firewall, PING, demo da última vaga): README, seção [Laboratório: três PCs (UEFS)](../README.md#laboratório-três-pcs-uefs). Cliente em outro SO: `SERVER_HOST=<IP do PC 1>` (PowerShell: `$env:SERVER_HOST`).
