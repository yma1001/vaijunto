# Decisões de projeto (revisáveis)

Não confundir com requisito oficial do enunciado. Hierarquia: enunciado > tutor > este arquivo > `CHAT.md`.

## DEC-001 — Linguagem Go

Status: CONFIRMADO (grupo)
Fonte: CHAT.md §9

Go pela stdlib de sockets (`net.Listen`/`Dial`), goroutines, `sync.RWMutex`, `go test -race` e `encoding/json`.

## DEC-002 — Interface CLI

Status: CONFIRMADO (grupo)
Fonte: CHAT.md §36

Dois executáveis de terminal: `cmd/driver` e `cmd/passenger`. Sem GUI.

## DEC-003 — TCP persistente + sessão na conexão

Status: CONFIRMADO (projeto)
Fonte: CHAT.md §18, §67.5

Após `LOGIN`, identidade e papel ficam na `net.Conn`. Sem token por requisição. Reinício do servidor descarta a sessão TCP; a conta persiste.

## DEC-004 — Framing length-prefix 4 bytes big-endian

Status: CONFIRMADO (projeto)
Fonte: CHAT.md §67.6

`[uint32 BE = N][N bytes JSON UTF-8]`. `MAX_PAYLOAD = 1 MiB` (limite técnico).

## DEC-005 — JSON UTF-8 como representação intermediária

Status: CONFIRMADO (projeto)
Fonte: CHAT.md §14, §29

Não usar `encoding/gob` na rede. Envelope com `version`, `operation`/`status`, `requestId`, `data`, `error`.

## DEC-006 — Protocolo único com autorização por papel

Status: CONFIRMADO (projeto)
Fonte: CHAT.md §17

Motorista e passageiro falam o mesmo protocolo. `DRIVER` vs `PASSENGER` restringe operações.

## DEC-007 — RWMutex global

Status: CONFIRMADO (projeto, primeira solução)
Fonte: CHAT.md §61, §71

Leituras: `RLock`. Escritas: `Lock`. Uma trava só evita deadlock por ordem de aquisição. Locks por trecho só se houver gargalo medido.

## DEC-008 — Grafo derivado, DFS de caminhos simples

Status: CONFIRMADO (projeto)
Fonte: CHAT.md §21–22

Cidades = vértices; trechos com vaga na data = arestas. Sem revisitar cidade. Ordenação: menor preço, depois menos baldeações, depois IDs.

Limites técnicos: `MAX_TRANSFERS=3`, `MAX_RESULTS=20`.

## DEC-009 — Sem horários intermediários / sem Maps

Status: CONFIRMADO (projeto)
Fonte: CHAT.md §24–25

A busca filtra pela data. Não calcula chegada nem rejeita conexão por horário.

## DEC-010 — Preço por trecho em centavos (`int64`)

Status: CONFIRMADO (projeto)
Fonte: CHAT.md §23; escolha local para evitar `float`

O motorista informa um preço por segmento em centavos (1500 = R$ 15,00). `totalPrice` é a soma.

## DEC-011 — Quantidade por trecho, sem assento numerado

Status: CONFIRMADO (projeto / quadro)
Fonte: CHAT.md §67.2

`0 <= availableSeats(segment) <= ride.capacity`.

## DEC-012 — Cancelamento do motorista = carona inteira

Status: CONFIRMADO (projeto)
Fonte: CHAT.md §67.3

Não há cancelamento de trecho isolado. Reservas afetadas viram `CANCELLED`. Em itinerário composto, assentos de outras caronas ativas são restaurados uma vez. Sem push; o passageiro vê na próxima listagem.

## DEC-013 — Idempotência

Status: CONFIRMADO (projeto)
Fonte: CHAT.md §67.4

`CANCEL_RESERVATION` repetido não devolve assento de novo. `CONFIRM_RESERVATION` usa `passengerId|requestId` como chave.

## DEC-014 — Persistência JSON

Status: CONFIRMADO (projeto)
Fonte: CHAT.md §35, §67.12

Arquivo `DATA_PATH` (padrão `data/state.json`). Escrita atômica (`tmp` + `rename`) ainda sob o `Lock`. Docker: volume `/data`. Sessão TCP não persiste.

## DEC-015 — Sem notificação push

Status: CONFIRMADO (projeto)
Fonte: CHAT.md §34

Protocolo request/response. Cliente consulta estado.

## DEC-016 — REGISTER no protocolo

Status: CONFIRMADO (projeto)
Fonte: escolha local para não depender só do seed

Operação `REGISTER` cria conta persistida (`DRIVER` ou `PASSENGER`). O seed (`motorista1`, `passageiro1`…) continua existindo para a demo.

## DEC-017 — Timeouts

Status: CONFIRMADO (projeto)

Idle da conexão: 10 min. Read/write de frame: 30 s. Connect: 10 s. O `RWMutex` nunca é retido durante I/O de rede.

## DEC-018 — Escuta em 0.0.0.0:5000

Status: CONFIRMADO (projeto)
Fonte: CHAT.md §8, §65

Não prender a `127.0.0.1` na demo multi-máquina. Clientes usam `SERVER_HOST`/`SERVER_PORT` = IP do host do servidor + porta publicada.

## DEC-019 — Relatório SBC em Markdown/LaTeX no repositório

Status: CONFIRMADO (projeto)

Texto em `relatorio/relatorio-sbc.md`, limitado ao conteúdo de 8 páginas. Compilação com o estilo SBC oficial fica a cargo do aluno se o `.sty` da SBC for usado.

## DEC-020 — Datas brasileiras só na CLI humana

Status: CONFIRMADO (projeto)
Fonte: usabilidade da demo; protocolo permanece ISO

`cmd/driver` e `cmd/passenger` pedem e mostram `DD/MM/AAAA`. Conversão testável em `internal/cliui` via `time.Parse`. O JSON da rede, os testes de protocolo e o `state.json` continuam `YYYY-MM-DD`. Data inválida (incluindo 31/02) gera mensagem amigável; não entra no fio.

## DEC-021 — Reais na CLI, centavos no protocolo

Status: CONFIRMADO (projeto)
Fonte: DEC-010; só muda a interface humana

A CLI pede um preço **por trecho** (`Preço X → Y (R$):`) e aceita `15`, `15,50` ou `15.50`. Internamente é `int64` centavos, **sem `float64`**. O JSON persistido e o protocolo **não** passam a usar decimal. Não se parseia uma linha com vários preços separados por vírgula (a vírgula brasileira seria ambígua). Exibição: `R$ 15,00`.

## DEC-022 — Cadastro no menu inicial reutiliza REGISTER

Status: CONFIRMADO (projeto)
Fonte: DEC-016

Menus iniciais: `1 entrar`, `2 criar conta`, `3 PING`, `0 sair`. Criar conta pede só usuário, senha e confirmação. O cliente motorista envia `role=DRIVER`; o passageiro, `PASSENGER`. Sem CPF, e-mail, telefone ou nome completo. O servidor gera o `userId`. Depois do cadastro a CLI mostra o ID e pede LOGIN; não autentica sozinha.

## DEC-023 — Sem API de roteamento / ETA nesta entrega

Status: CONFIRMADO (projeto)
Fonte: CHAT.md §24–25; análise em `docs/ROUTING_API.md`

Não há geocode, distância, ETA nem sugestão de preço por km neste código. Publicação manual funciona sozinha. Comparativo A (nada) / B (openrouteservice + env) / C (OSRM em outro container) está em `docs/ROUTING_API.md`. Recomendação desta entrega: A. Se no futuro houver API, ela será opcional, não bloqueará publicação, não rodará sob o `RWMutex` e a chave ficará só em variável de ambiente.
