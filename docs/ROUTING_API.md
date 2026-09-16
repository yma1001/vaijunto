# Análise: API de distância / ETA / sugestão de preço

**Status desta entrega:** não implementado. A publicação de carona continua 100% manual (rota, data, horário, assentos, preço por trecho informado pelo motorista).

Esta nota compara três caminhos. Não é requisito do enunciado. O protocolo, o `Ride` persistido e os testes atuais **não dependem** de geocodificação nem de roteamento.

Recomendação para **esta entrega / apresentação:** opção **A**. Se o grupo quiser experimentar depois, a opção **B** (openrouteservice via variável de ambiente) é a menos invasiva, sempre opcional.

## O que o enunciado realmente pede

O motorista informa **preço por trecho**. A busca combina trechos disponíveis na **data**. Não há obrigação de km, duração intermediária, ETA ou preço sugerido. Horários de chegada em cidades intermediárias **não existem** no modelo (DEC-009). Inventar uma API agora não cobre item do barema; cobre uma ideia de sessão.

## Opções

### A — Nenhuma API nesta entrega

- Publicar, buscar, reservar e cancelar funcionam offline, em LAN e em Docker, sem chave.
- Zero risco de a demo cair por quota, DNS ou bloqueio de saída.
- O aluno explica grafo + `RWMutex` + atomicidade sem desviar para HTTP externo.
- Não há preço/km nem ETA. O motorista continua dono do valor de cada trecho.

Custo: a “ideia Maps / valor por km” do quadro fica só como discussão.

### B — openrouteservice (ou similar) via ambiente

Serviço HTTP público (ou instância própria) para:

1. **Geocode** de nome de cidade → coordenada.
2. **Directions / routing** entre dois pontos → distância (m) e duração (s).

Chave: `ORS_API_KEY` (ou equivalente) **só em variável de ambiente**, nunca no git. Sem chave, o servidor **não chama** a API e a publicação manual segue igual.

Limites típicos da faixa gratuita: dezenas/centenas de requisições por minuto e um teto diário; precisa cache e degradação. Requer Internet no laboratório. Indisponibilidade (timeout, 429, 401) **não pode bloquear** publicar nem buscar.

Docker: o container do servidor precisaria de saída HTTPS. Não exige um segundo container.

### C — OSRM em outro container

Motor de roteamento self-hosted (OSM). Distância/duração entre coordenadas, em geral **sem chave**.

Exige: imagem OSRM, **mapa da região** (extract + `osrm-contract`), RAM, tempo de build, rede Docker entre `vaijunto-server` e `osrm`. Geocode **não vem** do OSRM: ainda seria Nominatim/Photon ou tabela fixa de cidades.

Offline no laboratório: possível **depois** de baixar o mapa. Na demo, se o container OSRM não subir, o sistema de caronas tem que continuar.

Mais operação para pouco ganho acadêmico neste PBL.

## Comparativo

| Tema | A — nada | B — ORS (env) | C — OSRM (container) |
|---|---|---|---|
| Geocode (cidade → lat/lon) | não | API de geocode do provedor | precisa de outro serviço (Nominatim etc.) |
| Distância / duração por trecho | não | directions HTTP | HTTP local ao container OSRM |
| Chaves / limites | nenhum | chave + quota | sem chave; custo de mapa/CPU |
| Offline / laboratório | sempre | falha sem Internet/chave | ok se o mapa já estiver no volume |
| Indisponibilidade | N/A | timeout → segue sem sugestão | container down → segue sem sugestão |
| Docker | inalterado | 1 container, egress HTTPS | 2+ containers + volume do mapa |
| Cache | N/A | cache por par de cidades (obrigatório na prática) | idem |
| Protocolo / `Ride` | inalterado | no máximo campos **opcionais** (`suggestedPrice`, `etaMinutes`) — hoje **não existem** | idem |
| Preço sugerido R$/km | não | `distance_m * taxa / 1000`, o motorista confirma ou ignora | igual, taxa local |
| ETA por trecho | não | duração da API, só informativo | igual |
| Testes | suíte atual basta | mock HTTP; nunca bater API real no `go test` | OSRM de teste é pesado; mock igual |
| Risco de prazo/apresentação | baixo | médio (rede, chave, demo) | alto (mapa, RAM, dois processos) |

## Regras se no futuro alguém implementar (não é agora)

1. **Opcional.** Publicar na mão continua válido sem API.
2. Indisponibilidade **não bloqueia** publicação, busca nem reserva.
3. **Nunca** chamar HTTP (nem DNS) com o `RWMutex` retido. Geocode/rota fora da seção crítica; o lock só grava o que o motorista confirmou.
4. Chave em env (`ORS_API_KEY`), listada no `.gitignore` se cair em arquivo; **nunca** commitada.
5. Estimativas são **sugestão**, não requisito do PBL. O preço persistido continua `int64` em centavos, informado (ou aceito) pelo motorista.
6. Cache por par normalizado de cidades para não estourar quota.
7. Testes com transport falso; a suíte de concorrência não depende de rede externa.

## Decisão

Para a entrega de 17/09/2026: **A**. O produto já demonstra TCP, protocolo, grafo, atomicidade e Docker. B ou C podem ser um extra **depois** da arguição, se o tutor pedir viabilidade temporal real (Q6 / DEC-009).
