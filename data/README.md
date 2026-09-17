# Diretório de estado persistido (JSON)

Não versionar o `state.json` de execução.

O servidor grava usuários, caronas, reservas e o índice de idempotência neste arquivo (`DATA_PATH`). Caminho relativo (`data/state.json`) é em relação ao **cwd**, não ao IP e não ao binário.

- Local: `export DATA_PATH="$PWD/data/state.json"` na raiz do clone.
- Docker: volume `vaijunto-data` montado em `/data` com `DATA_PATH=/data/state.json`. Esse volume **não** é este diretório do Git.

Os testes usam `t.TempDir()`; não apontam para cá.
