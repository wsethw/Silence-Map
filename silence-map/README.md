# Mapa de Silencio Colaborativo

Projeto de portfolio senior em Go, PostGIS e WebSocket para mapear silencio urbano em tempo real.

## Por que Go

Go combina baixa latencia, binario unico e concorrencia nativa. Para este produto, isso importa porque cada conexao WebSocket pode ficar aberta por muito tempo enquanto a API REST continua recebendo reports e confirmacoes. Goroutines e channels deixam o hub em tempo real simples, barato e previsivel.

## Por que PostGIS

PostGIS e melhor que MongoDB ou Elasticsearch para este nucleo porque consultas como "pontos em ate 5km", indices GIST em geografia, `ST_DWithin`, `ST_X`, `ST_Y` e agregacoes espaciais sao nativas no PostgreSQL. O modelo tambem precisa de integridade: UUIDs, foreign keys, `UNIQUE (report_id, user_id)` e transacoes ACID evitam confirmacoes duplicadas e dados quebrados.

## Confiabilidade do mapa

Cada report nasce com peso 1. Confirmacoes aumentam o peso colaborativo, mas um usuario nao pode confirmar o proprio report nem votar duas vezes no mesmo report. O banco aplica decaimento temporal com a funcao `temporal_decay`: reports ate 2 horas mantem peso 1.0, entre 2h e 24h caem gradualmente ate 0.5, e depois de 24h deixam de influenciar consultas de agora. Assim, um report antigo e isolado nao domina a regiao.

## Decisao sobre agregacao

O schema inclui a materialized view `place_aggregates` como ponto de partida para evoluir performance em alto volume. A API atual usa agregacao on-the-fly com `ST_DWithin` e `ST_SnapToGrid`, porque isso mantem a recomendacao sensivel a confirmacoes e ao decaimento em tempo real. Em producao, um job periodico poderia atualizar a view e o endpoint poderia combinar a view com reports recentes.

## Rodando

```powershell
cd C:\Users\codwj\OneDrive\Documentos\Project-04\silence-map
docker compose up --build
```

Abra [http://localhost:8080](http://localhost:8080).

## Teste por API

Crie cinco reports em Sao Paulo:

```powershell
curl.exe -s -X POST http://localhost:8080/api/reports -H "Content-Type: application/json" -d "{\"user_id\":\"u1\",\"latitude\":-23.5505,\"longitude\":-46.6333,\"quietness\":5,\"place_name\":\"Praça da Sé\"}"
curl.exe -s -X POST http://localhost:8080/api/reports -H "Content-Type: application/json" -d "{\"user_id\":\"u2\",\"latitude\":-23.5489,\"longitude\":-46.6388,\"quietness\":4,\"place_name\":\"Café Maravilha\"}"
curl.exe -s -X POST http://localhost:8080/api/reports -H "Content-Type: application/json" -d "{\"user_id\":\"u3\",\"latitude\":-23.5614,\"longitude\":-46.6559,\"quietness\":2,\"place_name\":\"Avenida Paulista\"}"
curl.exe -s -X POST http://localhost:8080/api/reports -H "Content-Type: application/json" -d "{\"user_id\":\"u4\",\"latitude\":-23.5570,\"longitude\":-46.6605,\"quietness\":5,\"place_name\":\"Biblioteca silenciosa\"}"
curl.exe -s -X POST http://localhost:8080/api/reports -H "Content-Type: application/json" -d "{\"user_id\":\"u5\",\"latitude\":-23.5432,\"longitude\":-46.6291,\"quietness\":3,\"place_name\":\"Rua movimentada\"}"
```

Capture um ID e confirme:

```powershell
$report = curl.exe -s -X POST http://localhost:8080/api/reports -H "Content-Type: application/json" -d "{\"user_id\":\"autor\",\"latitude\":-23.5505,\"longitude\":-46.6333,\"quietness\":5,\"place_name\":\"Ponto para confirmar\"}" | ConvertFrom-Json
curl.exe -s -X POST "http://localhost:8080/api/reports/$($report.id)/confirm" -H "Content-Type: application/json" -d "{\"user_id\":\"confirmador-1\"}"
```

Consulte reports recentes:

```powershell
curl.exe "http://localhost:8080/api/reports/recent?lat=-23.5505&lng=-46.6333&radius=5000"
```

Consulte lugares silenciosos usando o dia e hora atuais:

```powershell
$now = Get-Date
$isoDay = [int]$now.DayOfWeek
if ($isoDay -eq 0) { $isoDay = 7 }
curl.exe "http://localhost:8080/api/places/quiet?lat=-23.5505&lng=-46.6333&radius=5000&day_of_week=$isoDay&hour=$($now.Hour)"
```

Exemplo para sabado as 15h:

```powershell
curl.exe "http://localhost:8080/api/places/quiet?lat=-23.5505&lng=-46.6333&radius=5000&day_of_week=6&hour=15"
```

## Teste no navegador

1. Abra `http://localhost:8080`.
2. Clique no mapa e envie um report com nivel de silencio.
3. Abra outra aba no mesmo endereco.
4. Mova as duas abas para a mesma regiao do mapa.
5. Envie um report ou confirme um ponto em uma aba.
6. A outra aba recebe `new_report` ou `confirmation` via WebSocket se o ponto estiver dentro do bounding box visivel.

## Limite conhecido

O hub WebSocket e em memoria, ideal para um monolito de portfolio. Para escalar horizontalmente, varios processos Go precisariam compartilhar eventos por Redis Pub/Sub, NATS ou PostgreSQL `LISTEN/NOTIFY`, mantendo o filtro por bounding box em cada instancia.
