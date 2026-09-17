# VAIJUNTO

Sistema de **caronas compartilhadas** (PBL de Redes, Concorrência e Conectividade). Servidor central em Go fala **TCP/IP nativo** com um cliente **motorista** e um cliente **passageiro**. Itinerários podem juntar caronas diferentes. A reserva é **atômica** (tudo ou nada) com concorrência explícita (`goroutine` por conexão + `sync.RWMutex`). Estado em memória + arquivo JSON (sobrevive a reinício e a `docker rm` com volume).

Entrega: 17/09/2026.

## Contas da demo

Senha de todas: `senha123`

| usuário | papel |
|---|---|
| motorista1, motorista2 | DRIVER |
| passageiro1, passageiro2, passageiro3 | PASSENGER |

Também dá para **criar conta** no menu inicial de cada CLI (`2 criar conta`): só usuário, senha e confirmação. O servidor gera o ID. Sem CPF, e-mail ou telefone.

No **protocolo e no `state.json`** os preços continuam em **centavos** (`int64`, 1500 = R$ 15,00) e as datas em **YYYY-MM-DD**. Só a CLI pede data `DD/MM/AAAA` e preço em reais (`15`, `15,50` ou `15.50`), um trecho por vez.

## Pré-requisitos

O projeto está em [https://github.com/yma1001/vaijunto](https://github.com/yma1001/vaijunto). No laboratório da UEFS o fluxo é **baixar o ZIP** (navegador ou `curl`) — veja [Laboratório: três PCs (UEFS)](#laboratório-três-pcs-uefs). Não precisa de conta GitHub.

Para rodar os binários no próprio PC:

- **Go 1.22 ou mais novo** (`go.mod` declara `go 1.22`; confira com `go version`). Instale em [https://go.dev/dl/](https://go.dev/dl/).
- **Docker** para o servidor em container e para o laboratório (obrigatório no PC 1).

O caminho mais claro é **Linux** (laboratório da UEFS). macOS e Windows repetem os mesmos testes, com as diferenças de instalação, variáveis de ambiente e nome dos binários.

## Build e teste

Linux (primeiro caminho):

```bash
go test ./...
go test -race ./...
go build -o bin/server ./cmd/server
go build -o bin/driver ./cmd/driver
go build -o bin/passenger ./cmd/passenger
```

Se o Go do laboratório falhar (toolchain 1.22 indisponível), rode os testes numa imagem Docker — na pasta que contém `go.mod`:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.22 go test ./...
```

Disputa da última vaga (vários clientes TCP, só um confirma):

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.22 \
  go test ./internal/server -run TestSimultaneousConfirmTCP -v
```

Os mesmos `go test ./...` e `go test -race ./...` valem em macOS quando o Go local é 1.22+. No Windows, veja a seção Windows (`bin\*.exe` e `$env:...`).

## Rodar em um PC (Linux)

Três terminais. Contas da tabela acima; senha `senha123`.

Terminal 1:

```bash
cd /caminho/para/vaijunto-main
export DATA_PATH="$PWD/data/state.json"
./bin/server
# escuta 0.0.0.0:5000 — o log mostra o caminho absoluto do JSON
```

(`DATA_PATH=data/state.json ./bin/server` também vale **se** o cwd for a raiz do repositório. Relativo segue o diretório de trabalho, não o do binário.)

Terminal 2 (motorista):

```bash
export SERVER_HOST=127.0.0.1
export SERVER_PORT=5000
./bin/driver
# menu: 1 entrar  2 criar conta  3 PING  0 sair
# publicar: cidades livres (mínimo 2), data DD/MM/AAAA, preço por trecho em R$
```

Terminal 3 (passageiro):

```bash
export SERVER_HOST=127.0.0.1
export SERVER_PORT=5000
./bin/passenger
```

Smoke automático (última vaga + persistência):

```bash
bash scripts/smoke.sh
```

Cliente em outra linguagem:

```bash
python3 examples/python_ping.py 127.0.0.1 5000
```

## Persistência e DATA_PATH (não confundir com IP)

O JSON **não** guarda o IP do servidor. `SERVER_HOST` só diz ao **cliente** onde conectar. Trocar de rede, voltar para casa ou anotar outro IP **não** deve apagar reservas.

O que parece “sumiu tudo, mas passageiro1 ainda entra” é quase sempre **dois arquivos** com o mesmo seed (`motorista1` / `passageiro1`…):

| Como sobe o servidor | Arquivo |
|---|---|
| `./bin/server` na pasta do projeto, `DATA_PATH=data/state.json` | `<pasta>/data/state.json` (relativo ao **cwd**) |
| `./bin/server` iniciado de outro diretório com o mesmo relativo | **outro** `data/state.json` naquele cwd |
| Docker (`docker compose` / `docker run -v vaijunto-data:/data`) | volume `vaijunto-data` → `/data/state.json` **dentro** do container |

Os dois têm as contas seed, então o LOGIN funciona. Caronas e reservas ficaram no arquivo que **não** está aberto agora.

**Ubuntu / Linux (laboratório ou em casa)** — fixe um caminho absoluto e suba sempre o mesmo processo/volume:

```bash
cd /caminho/para/vaijunto-main
export DATA_PATH="$PWD/data/state.json"
./bin/server
```

O log na subida mostra `data=<caminho absoluto>`. Depois de um restart, no cliente passageiro: `LOGIN` de novo e **3) minhas reservas** (isso manda `LIST_RESERVATIONS` ao servidor; o cache local da CLI não sobrevive a restart/logout).

**Windows (binário local):** na pasta extraída do ZIP, prefira caminho absoluto:

```powershell
$env:DATA_PATH = (Join-Path (Get-Location) "data\state.json")
.\bin\server.exe
```

**Windows / Linux / macOS + Docker:** use **sempre** o volume nomeado e `DATA_PATH=/data/state.json`. O IP (`SERVER_HOST`) muda; o volume não.

```bash
docker run -d --name vaijunto-server --restart unless-stopped \
  -p 5000:5000 -v vaijunto-data:/data \
  -e LISTEN_HOST=0.0.0.0 -e SERVER_PORT=5000 -e DATA_PATH=/data/state.json \
  vaijunto-server
```

Não misture `.\bin\server.exe` + `data\state.json` com esse container esperando as mesmas reservas. `docker compose down` **sem** `-v` mantém `vaijunto-data`. Não use `down -v` nem apague o volume se quiser conservar o estado.

Testes e smoke usam `DATA_PATH` temporário; não apontam para o `data/state.json` do aluno.

## macOS

Instale o Go com Homebrew (`brew install go`) ou pelo pacote em [https://go.dev/dl/](https://go.dev/dl/). `go version` deve ser 1.22 ou mais novo.

Na pasta extraída do ZIP, testes e build são iguais aos do Linux. Variáveis: os mesmos `export SERVER_HOST`, `SERVER_PORT` e `DATA_PATH`.

Três terminais. Senha da demo: `senha123`.

Terminal 1:

```bash
cd /caminho/para/vaijunto-main
export DATA_PATH="$PWD/data/state.json"
./bin/server
```

Terminal 2:

```bash
export SERVER_HOST=127.0.0.1
export SERVER_PORT=5000
./bin/driver
```

Terminal 3:

```bash
export SERVER_HOST=127.0.0.1
export SERVER_PORT=5000
./bin/passenger
```

Smoke: `bash scripts/smoke.sh` (funciona no Terminal do macOS).

Docker: [Docker Desktop](https://www.docker.com/products/docker-desktop/). Com o daemon no ar, os comandos da seção Docker abaixo são os mesmos.

## Windows

Instale o Go (msi) em [https://go.dev/dl/](https://go.dev/dl/) e o [Git for Windows](https://git-scm.com/download/win) (traz Git Bash). `go version` ≥ 1.22.

O **Git Bash** segue os comandos Linux (`export`, `./bin/server`, `bash scripts/smoke.sh`).

No **PowerShell**, as variáveis são `$env:SERVER_HOST`, `$env:SERVER_PORT` e `$env:DATA_PATH` (uma janela não herda a da outra). Os binários ficam em `bin\` com sufixo `.exe`.

```powershell
# extraia o ZIP do GitHub (Code → Download ZIP); a pasta fica vaijunto-main
cd vaijunto-main
go test ./...
go test -race ./...
go build -o bin/server.exe ./cmd/server
go build -o bin/driver.exe ./cmd/driver
go build -o bin/passenger.exe ./cmd/passenger
```

Se `go test -race` falhar pedindo CGO/gcc, instale um gcc (MinGW-w64) ou rode no Git Bash/WSL. `go test ./...` continua válido.

Três janelas do PowerShell. Senha da demo: `senha123`.

Janela 1 (servidor):

```powershell
$env:DATA_PATH = (Join-Path (Get-Location) "data\state.json")
.\bin\server.exe
# escuta 0.0.0.0:5000
```

Janela 2 (motorista):

```powershell
$env:SERVER_HOST = "127.0.0.1"
$env:SERVER_PORT = "5000"
.\bin\driver.exe
```

Janela 3 (passageiro):

```powershell
$env:SERVER_HOST = "127.0.0.1"
$env:SERVER_PORT = "5000"
.\bin\passenger.exe
```

Smoke: no Git Bash, `bash scripts/smoke.sh`. No PowerShell nativo o script bash não roda; use `go test ./...` e a demo dos três `.exe`.

Firewall: em `127.0.0.1` em geral não pede nada. Se outro PC não conectar, libere **TCP 5000** no Windows Defender Firewall (e permita o `server.exe` se o Defender perguntar).

Docker: Docker Desktop para Windows. `docker build` / `docker compose` no PowerShell; `bash scripts/docker-local.sh` no Git Bash.

Cliente Python: `python examples/python_ping.py 127.0.0.1 5000`.

## Docker (mesma máquina)

Linux: Docker Engine. macOS e Windows: Docker Desktop. O script abaixo é bash (no Windows, Git Bash).

Script único (build + smoke + `docker restart` + persistência). O container do servidor permanece no ar:

```bash
bash scripts/docker-local.sh
```

Equivalente manual:

```bash
docker build -t vaijunto-server --build-arg BUILD_TARGET=server .
docker compose up -d
# volume vaijunto-data → /data/state.json
```

Clientes no host apontam para `127.0.0.1:5000`.

## Laboratório: três PCs (UEFS)

**PC 1 = servidor.** **PC 2 = passageiro.** **PC 3 = motorista.**

A conversa é TCP na porta **5000**. **PC 2 e PC 3 sempre usam o IP LAN real do PC 1.**

Nos comandos de PCs diferentes o placeholder é `172.16.XXX.X` — **modelo visual**, não um IP para copiar. Veja [Qual IP usar](#qual-ip-usar-server_host).

```text
PC 3  motorista
        \
         \  TCP
          \
      172.16.XXX.X:5000
          PC 1  servidor  (Docker  -p 5000:5000)
          /
         /  TCP
        /
PC 2  passageiro
```

O laboratório é **Linux**. Não rode `go run ./cmd/server` nem `./bin/server` no PC 1 se o Compose já estiver no ar: mesma porta **e** dois `state.json` diferentes.

Contas seed (senha `senha123`): `motorista1`/`motorista2`, `passageiro1`/`passageiro2`/`passageiro3`. Menu: `1 entrar` · `2 criar conta` · `3 PING` · `0 sair`.

Se `docker` pedir permissão, use `sudo docker …` / `sudo docker compose …`.

### Como pegar o projeto (qualquer PC)

Só ZIP. Repositório público: não precisa de conta GitHub.

**Navegador:** [https://github.com/yma1001/vaijunto](https://github.com/yma1001/vaijunto) → botão verde **Code** → **Download ZIP** → extraia.

**Terminal:**

```bash
curl -L -o vaijunto-main.zip https://github.com/yma1001/vaijunto/archive/refs/heads/main.zip
unzip -o vaijunto-main.zip
cd vaijunto-main
```

Cheque se está na pasta **certa** (os arquivos abaixo têm que estar **direto** nela, não num nível acima):

```bash
pwd
ls
ls docker-compose.yml go.mod
```

Tem que listar, no mesmo diretório:

- `Dockerfile`
- `docker-compose.yml`
- `go.mod`
- `cmd/`
- `internal/`

**Pasta dentro de pasta.** Se você baixar/descompactar o ZIP **já estando** dentro de `vaijunto-main`, pode ficar:

```text
~/Downloads/vaijunto-main/vaijunto-main
```

Isso não é necessariamente erro. Os comandos (`docker compose`, `docker build`, `go run`) têm que rodar na pasta **interna** que realmente contém `docker-compose.yml`, `go.mod`, `cmd/` e `internal/`. Se `ls docker-compose.yml go.mod` falhar, entre um nível (`cd vaijunto-main`) e teste de novo.

Para atualizar: baixe o ZIP outra vez. Extraia **fora** da pasta antiga, ou confira o `pwd` antes de `cd`.

### Qual IP usar (`SERVER_HOST`)

**PC 1 = servidor.** **PC 2 = passageiro.** **PC 3 = motorista.**  
PC 2 e PC 3 **sempre** apontam para o IP LAN **real** do PC 1.

O modelo nos comandos é:

```text
172.16.XXX.X
```

Isso é **só um desenho**. Não digite `XXX` nem copie o modelo. As salas da UEFS usam sub-redes diferentes (`172.16.103.X`, `172.16.201.X`, …): o terceiro octeto (`XXX`) muda; o último `X` é o número da máquina.

O IP real do PC 1 sai de:

```bash
hostname -I
```

Rode isso **no PC servidor**, depois que o container estiver no ar.

**ATENÇÃO:** `172.16.XXX.X` é só um modelo. Rode `hostname -I` no PC servidor e coloque o IP LAN real mostrado ali.

Exemplos (não copie se o seu PC 1 mostrou outro número):

| `hostname -I` no PC 1 | `SERVER_HOST` nos PCs 2 e 3 |
|---|---|
| `172.16.103.10 172.17.0.1` | `SERVER_HOST=172.16.103.10` |
| `172.16.201.8 172.17.0.1` | `SERVER_HOST=172.16.201.8` |

Na lista do `hostname -I`, o da LAN costuma ser o `172.16.…` que **não** termina em `.0.1`. Os `172.17.0.1`, `172.18.0.1`, `172.19.0.1` são bridges internas do Docker — **não** use.

O PC 2 pode ser `172.16.103.9`. Ele **não** coloca o próprio IP no `SERVER_HOST`. Continua usando o do PC 1 (ex. `172.16.103.10`).

```text
PC 3  motorista
        \
         \  TCP
          \
      172.16.XXX.X:5000     ← IP LAN real do PC 1 (hostname -I)
          PC 1  servidor
          /
         /  TCP
        /
PC 2  passageiro
```

Não use:

- o IP do próprio PC 2
- o IP do próprio PC 3
- `172.17.x.x` / `172.18.x.x` / `172.19.x.x` das bridges Docker
- `127.0.0.1` quando cliente e servidor estão em **computadores diferentes**

Dois casos:

**Mesmo computador** (teste no PC 1, outro terminal):

```bash
SERVER_HOST=127.0.0.1 SERVER_PORT=5000 go run ./cmd/passenger
```

Se o passageiro estiver **em container no mesmo PC** do servidor, `127.0.0.1` dentro do container é o próprio container. Use `--network host`:

```bash
docker run -it --rm --network host \
  -e SERVER_HOST=127.0.0.1 \
  -e SERVER_PORT=5000 \
  vaijunto-passenger
```

**PCs diferentes** (laboratório):

```text
SERVER_HOST=172.16.XXX.X
```

Substitua pelo IP LAN real do PC 1 (`hostname -I`). Não digite `XXX`.

### PC 1 — servidor

Na pasta correta (`ls docker-compose.yml go.mod` ok):

```bash
docker compose config --services
```

Neste repositório o esperado é:

```text
server
```

O Compose **só** declara o serviço `server`. Passageiro e motorista **não** sobem com `docker compose up`.

```bash
docker compose build
docker compose up -d server
docker compose ps
```

`docker compose ps` deve mostrar a porta do **computador** publicada, algo como:

```text
0.0.0.0:5000->5000/tcp
```

(ou `:::5000->5000/tcp`). Sem isso, PC 2 e PC 3 não entram.

```bash
hostname -I
```

Anote o **IP da LAN do PC 1** (`172.16.…` que não é bridge Docker). Esse número entra no `SERVER_HOST` dos PCs 2 e 3 no lugar de `172.16.XXX.X`.

Log (escuta `0.0.0.0:5000`; o log mostra `data=/data/state.json`):

```bash
docker compose logs -f server
```

`Ctrl+C` sai dos logs e **não** derruba o servidor.

Teste **no mesmo PC 1**, outro terminal (`127.0.0.1` vale **só aqui**):

```bash
SERVER_HOST=127.0.0.1 SERVER_PORT=5000 go run ./cmd/passenger
```

Se o Go falhar e o passageiro for um container **neste mesmo PC**, use `--network host` (veja [Qual IP usar](#qual-ip-usar-server_host)). Menu `3) PING` → `PONG` confirma que o servidor aceita TCP. `0` sai.

### PC 2 — passageiro

ZIP extraído; pasta com `go.mod` e `cmd/`. Primeiro:

```bash
go version
```

Se o Go estiver ok (1.22+) e aceitar o módulo:

> **ATENÇÃO:** `172.16.XXX.X` é só um modelo. Rode `hostname -I` no PC servidor e coloque o IP LAN real mostrado ali.

```bash
SERVER_HOST=172.16.XXX.X SERVER_PORT=5000 go run ./cmd/passenger
```

Erro **real** visto no laboratório:

```text
go: downloading go1.22 (linux/amd64)
go: download go1.22 for linux/amd64: toolchain not available
```

Isso **não** é TCP, **não** é IP e **não** é bug do VAIJUNTO. O Go daquela máquina não conseguiu usar/baixar a toolchain 1.22 do `go.mod`. **Não perca tempo consertando o Go do laboratório.** Use Docker:

> **ATENÇÃO:** `172.16.XXX.X` é só um modelo. Rode `hostname -I` no PC servidor e coloque o IP LAN real mostrado ali.

```bash
docker build -t vaijunto-passenger --build-arg BUILD_TARGET=passenger .

docker run -it --rm \
  -e SERVER_HOST=172.16.XXX.X \
  -e SERVER_PORT=5000 \
  vaijunto-passenger
```

A tela deve mostrar o IP **real** do PC 1 (ex. `Servidor: 172.16.103.10:5000`). Primeiro: **`3) PING`**. Se vier `PONG`, o PC 2 está falando com o container do PC 1. Depois: `1) entrar` com `passageiro1` / `senha123`.

**`docker compose ps` vazio no PC 2 é normal.** O `docker-compose.yml` só tem o serviço `server`, e o servidor roda **no PC 1**. No PC 2 o cliente é `go run ./cmd/passenger` **ou** a imagem `BUILD_TARGET=passenger`. Não precisa haver container listado no Compose do PC 2.

### PC 3 — motorista

Mesma pasta correta do ZIP. Mesmo IP LAN do PC 1 que o passageiro usou.

> **ATENÇÃO:** `172.16.XXX.X` é só um modelo. Rode `hostname -I` no PC servidor e coloque o IP LAN real mostrado ali.

Se o Go funcionar:

```bash
SERVER_HOST=172.16.XXX.X SERVER_PORT=5000 go run ./cmd/driver
```

Se aparecer o mesmo erro de toolchain (`download go1.22` / `toolchain not available`):

```bash
docker build -t vaijunto-driver --build-arg BUILD_TARGET=driver .

docker run -it --rm \
  -e SERVER_HOST=172.16.XXX.X \
  -e SERVER_PORT=5000 \
  vaijunto-driver
```

**`docker compose ps` vazio no PC 3 também é normal.**

Primeiro `3) PING`. Login: `motorista1` / `senha123`. Publicar: cidades, data `DD/MM/AAAA`, hora, assentos, preço por trecho em R$.

### Testes automáticos se o Go do laboratório não funciona

Na pasta com `go.mod`:

```bash
docker run --rm \
  -v "$PWD":/src \
  -w /src \
  golang:1.22 \
  go test ./...
```

Disputa da última vaga (vários clientes TCP; o correto é **um** sucesso):

```bash
docker run --rm \
  -v "$PWD":/src \
  -w /src \
  golang:1.22 \
  go test ./internal/server \
  -run TestSimultaneousConfirmTCP -v
```

### Resumo

**PC 1**

```bash
docker compose build
docker compose up -d server
docker compose ps
hostname -I
```

Anote o IP LAN (`172.16.…`, não `172.17`/`172.18`/`172.19`). Nos PCs 2 e 3 ele entra no lugar de `172.16.XXX.X`.

> **ATENÇÃO:** `172.16.XXX.X` é só um modelo. Rode `hostname -I` no PC servidor e coloque o IP LAN real mostrado ali.

**PC 2**

```bash
docker build -t vaijunto-passenger --build-arg BUILD_TARGET=passenger .
docker run -it --rm \
  -e SERVER_HOST=172.16.XXX.X \
  -e SERVER_PORT=5000 \
  vaijunto-passenger
```

**PC 3**

```bash
docker build -t vaijunto-driver --build-arg BUILD_TARGET=driver .
docker run -it --rm \
  -e SERVER_HOST=172.16.XXX.X \
  -e SERVER_PORT=5000 \
  vaijunto-driver
```

Não use:

- o IP do próprio PC 2
- o IP do próprio PC 3
- `127.0.0.1` nos PCs 2 e 3 (isso é **mesmo PC**)
- `172.17.x.x` / `172.18.x.x` / `172.19.x.x` das bridges Docker

### Se der `connection refused`

No **PC 1**:

```bash
docker compose ps
docker compose logs server
ss -ltn | grep 5000
```

Tem que haver algo em `*:5000` ou `0.0.0.0:5000`. Se o Compose não estiver `Up`: `docker compose up -d server`.

```bash
sudo ufw status
```

Se estiver `active` e houver permissão:

```bash
sudo ufw allow 5000/tcp
```

### Se der `i/o timeout` ou o PING não volta

IP errado, PCs em redes diferentes, ou porta 5000 bloqueada. Rode `hostname -I` de novo **no PC 1** e coloque esse IP LAN no `SERVER_HOST` dos PCs 2 e 3 (no lugar de `172.16.XXX.X`).

### Demo que vale na arguição

1. PC 3 publica Salvador → Feira de Santana → Jequié com **1** assento.
2. PC 2 (`passageiro1`) busca e confirma.
3. Outro passageiro (segundo terminal no PC 2, ou outro PC) busca a mesma rota e tenta confirmar.

Esperado: **um** OK e o outro `NO_SEATS` / “sem vagas”. Nunca os dois confirmados com capacidade 1.

Depois: `docker compose restart` no PC 1 → LOGIN de novo; no passageiro, menu **`3) minhas reservas`**. O volume `vaijunto-data` guarda o JSON; a sessão TCP não.

Parar o servidor **sem apagar** contas/caronas: `docker compose down` no PC 1. **Nunca** `docker compose down -v`.

### Equivalente sem Compose no PC 1

```bash
docker build -t vaijunto-server --build-arg BUILD_TARGET=server .
docker run -d --name vaijunto-server --restart unless-stopped \
  -p 5000:5000 -v vaijunto-data:/data \
  -e LISTEN_HOST=0.0.0.0 -e SERVER_PORT=5000 -e DATA_PATH=/data/state.json \
  vaijunto-server
```

O `-p 5000:5000` publica a porta: 5000 no computador → 5000 no container. Sem isso, só a rede interna do Docker alcança o processo.

## Documentação

| arquivo | conteúdo |
|---|---|
| `docs/PROTOCOL.md` | protocolo completo (dá para escrever um cliente Python) |
| `docs/ARCHITECTURE.md` | componentes e modelo |
| `docs/CONCURRENCY.md` | locks, atomicidade, invariantes |
| `docs/DECISIONS.md` | decisões de projeto vs enunciado |
| `docs/TESTING.md` | como testar |
| `docs/ROUTING_API.md` | análise (não implementada) de geocode/ETA |

## Layout do código

```text
cmd/server driver passenger loadtest smoke
internal/protocol domain store search service server client config cliui
```

Comentários no código explicam **por que** (framing, lock, grafo), não a sintaxe.

## Invariantes

Disponibilidade nunca negativa nem acima da capacidade; reserva composta atômica; cancelamento restaura uma vez; busca não segura vaga; cliente morto não derruba o servidor; JSON inválido não muta estado; dados persistidos reaparecem após restart.
