# VAIJUNTO: caronas compartilhadas com sockets TCP e concorrência explícita

**PBL — MI Concorrência e Conectividade / Redes de Computadores**  
Yago Mendes e demais membros do grupo (UEFS)  
Entrega: 17/09/2026

Este texto está dimensionado para o limite de **8 páginas** do formato SBC. Pode ser colado no template oficial (`sbc-template`) para gerar o PDF. Sem o `.sty` oficial da SBC neste repositório, o PDF não é compilado — o entregável versionado é este Markdown.

---

## Resumo

Este trabalho apresenta o VAIJUNTO, um sistema de caronas intermunicipais com servidor central em Go. A comunicação usa sockets TCP/IP e um protocolo de aplicação próprio (framing de tamanho + JSON UTF-8). A disponibilidade de assentos é controlada por trecho. Itinerários podem combinar caronas. A confirmação é atômica sob um `sync.RWMutex`. O estado persiste em JSON e os componentes executam em Docker, inclusive em máquinas distintas.

**Palavras-chave:** TCP, sockets, concorrência, goroutine, mutex, reserva atômica, Docker.

## 1. Introdução

O problema pede um serviço no estilo de caronas de longa distância: o motorista publica uma rota ordenada de cidades, data, horário, capacidade e preço por trecho; o passageiro busca origem/destino/data e reserva. Como o passageiro pode embarcar e desembarcar em cidades intermediárias, a vaga não pode ser um único contador da viagem inteira. Como dois passageiros podem confirmar ao mesmo tempo, a aplicação — e não um banco — deve garantir exclusão mútua e atomicidade. Como a demonstração ocorre em laboratório, servidor e clientes precisam falar TCP em computadores diferentes, empacotados em containers.

O restante do relatório segue os eixos do barema: arquitetura, comunicação, protocolo, encapsulamento, busca, concorrência, atomicidade, interação, confiabilidade, testes e emulação.

## 2. Arquitetura

Há três programas. O **servidor** é o único processo com estado. Os **clientes** motorista e passageiro são CLIs que abrem uma conexão TCP persistente. O estado canônico (usuários, caronas, trechos, reservas) vive em memória protegido por `RWMutex` e é gravado em `state.json` após cada mutação. O grafo de busca é *derivado* das caronas ativas no instante da consulta, evitando uma segunda estrutura a sincronizar.

Uma carona com cidades \(C_0,\ldots,C_n\) gera \(n\) segmentos \(C_i \rightarrow C_{i+1}\), cada um com `availableSeats` inicial igual à capacidade. Duas caronas com a mesma rota permanecem entidades distintas. A sessão autenticada pertence à conexão: não é persistida. Contas e reservas são.

## 3. Comunicação e protocolo

O transporte é TCP porque a reserva precisa de canal confiável e ordenado. O servidor faz `Listen` em `0.0.0.0:5000`, `Accept` em loop e lança uma goroutine por `net.Conn`. O cliente faz `Dial(SERVER_HOST, SERVER_PORT)`. Não há HTTP, gRPC nem fila de mensagens: o enunciado exige a interface de sockets.

Como TCP é um fluxo, o protocolo define *framing*: quatro bytes big-endian com o tamanho \(N\) do payload, seguidos de \(N\) bytes JSON UTF-8 (`MAX_PAYLOAD` = 1 MiB). O receptor usa leitura exata (`io.ReadFull`) e rejeita \(N=0\) ou \(N\) absurdo sem alocar o corpo. O JSON é a representação intermediária: um cliente Python (`examples/python_ping.py`) interpreta as mesmas mensagens.

O envelope traz `version`, `operation` ou `status`, `requestId` e `data`. Há um único catálogo de operações; o papel `DRIVER`/`PASSENGER` autoriza. `LOGIN` liga a identidade à conexão. Códigos estáveis (`NO_SEATS`, `INVALID_JSON`, `UNAUTHENTICATED`, …) permitem tratar erro sem parsear texto livre. A especificação completa, com exemplos, está em `docs/PROTOCOL.md`.

## 4. Encapsulamento e validação

O encapsulamento tem duas camadas: o frame delimita a mensagem no fluxo; o JSON estrutura os campos. Chegada: (1) ler header; (2) validar \(N\); (3) ler payload; (4) `json.Unmarshal`; (5) validar `version=1`, `operation` e `requestId`; (6) decodificar `data` da operação. JSON malformado devolve `INVALID_JSON` e não altera o Store. Frame incompleto encerra só aquela conexão. Operação desconhecida devolve `UNKNOWN_OPERATION` e o Accept loop continua.

## 5. Busca de itinerários

Cada trecho com vaga na data pedida vira aresta dirigida entre cidades, etiquetada com `rideId` e preço. Uma DFS em caminhos simples (cidade não se repete) encontra rotas diretas e compostas. Segmentos consecutivos da mesma carona são compactados num único *leg*. Ordenação: menor `totalPrice`, depois menos baldeações, depois identificadores — resultado determinístico. Limites técnicos (`MAX_TRANSFERS=3`, `MAX_RESULTS=20`) evitam explosão combinatória; não são requisito do enunciado.

A política temporal é deliberadamente mínima: filtra-se a **data**. O enunciado não define chegada nas cidades intermediárias; não se inventou duração nem se integrou API de mapas.

A busca é *snapshot* sob `RLock`. Ver vaga não a reserva.

## 6. Concorrência e atomicidade

Duas camadas: goroutines atendem clientes em paralelo; o `RWMutex` protege o mapa compartilhado. Leituras (`SEARCH`, listagens) usam `RLock`. Escritas (`PUBLISH`, `CONFIRM`, cancelamentos) usam `Lock`. A trava nunca é retida durante I/O de rede.

A primeira solução correta usa **um** RWMutex global. Com uma trava, confirmar um itinerário de várias caronas entra numa única seção crítica, o que implementa a atomicidade e elimina deadlock por ordem de aquisição (o ciclo clássico “Ride1 espera Ride2 e vice-versa” não existe). Isso equivale a adquirir todas as travas de trecho simultaneamente.

Algoritmo de `CONFIRM_RESERVATION`: (1) chave de idempotência `passengerId|requestId`; (2) expandir cada leg em índices de segmento; (3) revalidar `availableSeats >= 1` em todos; (4) se algum falhar, responder `NO_SEATS` sem mutar; (5) decrementar todos, gravar `Reservation`, persistir JSON. Dois passageiros e uma vaga: um `OK` e um `NO_SEATS`. Vence quem o servidor processa primeiro na seção crítica — interpretação operacional de “quem confirma primeiro”.

Cancelar reserva restaura cada segmento ativo exatamente uma vez. Cancelar carona marca a carona e as reservas afetadas como `CANCELLED` e devolve assentos de outras caronas ainda ativas em itinerários compostos. Não há *push*; o passageiro vê o estado na próxima listagem.

## 7. Interação

O cliente motorista autentica, publica rota/data/hora/capacidade/preços, lista caronas, lista passageiros confirmados **por trecho** e cancela a carona inteira. O cliente passageiro busca, confirma um itinerário da última busca, lista e cancela reservas. Ambos fazem `PING` sem login, útil para testar a LAN.

## 8. Confiabilidade

`defer recover` isola pânico da conexão. Cliente que fecha no meio do frame, envia JSON inválido ou `N` gigante não derruba o processo nem deixa lock preso. Timeouts: idle 10 min, write 30 s. Se a reserva já foi commitada e a escrita da resposta falha, repetir o mesmo `requestId` devolve a mesma reserva. Persistência usa arquivo temporário + `rename` atômico, ainda sob o `Lock` de escrita, para não intercalar JSON. Volume Docker guarda `/data`. A sessão TCP morre no restart; as contas não.

## 9. Testes

A suíte cobre framing (leitura parcial, mensagens coalescidas, frame incompleto), busca direta e composta, disputa da última vaga com 32 goroutines, confirmação simultânea em sockets TCP reais, último trecho indisponível sem reserva parcial, cancelamento repetido, `requestId` repetido, muitos clientes, JSON inválido, operação desconhecida, desconexão abrupta, invariantes de capacidade e reinício com o mesmo `state.json`. Executam-se `go test ./...` e `go test -race ./...`. O `cmd/loadtest` mede sucessos, falhas, média, p95 e *throughput* com clientes TCP reais. Não se promete limite de latência que o enunciado não fixou.

## 10. Emulação com Docker

Cada binário é uma imagem (`BUILD_TARGET=server|driver|passenger`). O servidor publica `-p 5000:5000` e monta um volume em `/data`. Em três PCs, o ponto fácil de errar é o endereço: o cliente deve usar o **IP do host** do PC servidor na LAN, não o IP do bridge (`172.x`). Vantagens: o laboratório não precisa instalar a toolchain Go; o ambiente é reproduzível; o volume sobrevive à recriação do container. Não se usa overlay, Swarm nem Kubernetes.

## 11. Conclusão

O VAIJUNTO demonstra, no mesmo protótipo, os dois eixos da disciplina: um protocolo de aplicação sobre TCP com representação intermediária validada, e um estado compartilhado cuja correção se discute em termos de seção crítica, atomicidade, corrida e deadlock. As decisões que o enunciado não fecha (JSON, RWMutex global, persistência em arquivo, ausência de push e de Maps) estão registradas como decisões de projeto, não como requisitos oficiais.

## Referências

1. Enunciado do Problema 1 — VAIJUNTO (material da disciplina).
2. Barema do Problema 1 (11 itens de arguição).
3. Documentação do projeto: `docs/PROTOCOL.md`, `docs/CONCURRENCY.md`, `docs/ARCHITECTURE.md`.
4. Go standard library: pacotes `net`, `sync`, `encoding/json`, `encoding/binary`.
