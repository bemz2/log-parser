# log-parser

`log-parser` - микросервис на Go для загрузки логов сетевой топологии, разбора узлов и портов, сохранения результата в PostgreSQL и отдачи данных через REST API.

## Технологии

- Go 1.26
- `net/http` и `http.ServeMux`
- PostgreSQL
- `database/sql` + `pgx/v5` driver
- Docker Compose
- `log/slog`

## Архитектура

Запрос `POST /api/v1/parse` проходит через HTTP handler, service валидирует путь и запускает parser, repository сохраняет результат в PostgreSQL.

Основные правила:

- читать можно только файлы внутри `data/`;
- поддержаны расширения `.log`, `.txt`, `.csv`, `.db_csv`, `.sharp_an_info`;
- если парсер находит ошибку, запись `logs` получает статус `failed`, а `nodes`, `ports`, `nodes_info` не сохраняются;
- сохранение распарсенных данных выполняется в SQL transaction;
- миграции применяются автоматически при старте приложения.

## Структура

```text
cmd/log-parser/              entrypoint
internal/application/             сборка приложения и HTTP server
internal/client/postgres/         подключение к PostgreSQL
internal/delivery/http/           handlers, responses, middleware
internal/domain/                  доменные модели
internal/migration/               встроенный мигратор
internal/parser/                  построчный parser package
internal/repository/              SQL repository
internal/service/                 бизнес-логика и валидация путей
migrations/                       SQL migrations
data/                             директория логов
postman_collection.json           Postman collection
```

## Запуск

```bash
cp .env.sample .env
docker compose up -d
```

После запуска API доступно на:

```text
http://localhost:8080
```

Проверка:

```bash
curl http://localhost:8080/health
```

## Makefile

```bash
make setup       # подготовить .env, скачать зависимости, поднять postgres
make run         # локальный запуск приложения
make test        # unit и handler integration tests
make mocks       # сгенерировать mockery mocks
make build       # собрать bin/log-parser
make compose-up  # собрать и поднять app + postgres
```

## ENV

| Переменная | Значение по умолчанию |
| --- | --- |
| `PORT` | `8080` |
| `DB_HOST` | `localhost` |
| `DB_PORT` | `5432` |
| `DB_USER` | `topology` |
| `DB_PASSWORD` | `topology` |
| `DB_NAME` | `topology` |
| `DATA_DIR` | `data` |
| `MIGRATIONS_DIR` | `migrations` |

## API

### Parse

```bash
curl -X POST http://localhost:8080/api/v1/parse \
  -H 'Content-Type: application/json' \
  -d '{"path":"data/ibdiagnet2.db_csv"}'
```

Ответ:

```json
{
  "log_id": 1,
  "status": "parsed"
}
```

### Topology

```bash
curl http://localhost:8080/api/v1/topology/1
```

Ответ:

```json
{
  "log_id": 1,
  "nodes": [
    {
      "id": 1,
      "external_id": "0xswitch1",
      "name": "SWITCH_1",
      "type": "switch",
      "ports": [
        {
          "id": 1,
          "name": "port-1",
          "ip": "1",
          "status": "active",
          "speed": "2048"
        }
      ]
    }
  ]
}
```

### Node

```bash
curl http://localhost:8080/api/v1/node/1
```

Возвращает node, ports и nodes_info.

### Ports

```bash
curl http://localhost:8080/api/v1/port/1
```

Возвращает список портов узла.

### Log

```bash
curl http://localhost:8080/api/v1/log/1
```

Ответ:

```json
{
  "id": 1,
  "file_path": "data/ibdiagnet2.db_csv",
  "status": "parsed",
  "nodes_count": 5,
  "ports_count": 151,
  "uploaded_at": "2026-05-15T10:00:00Z"
}
```

## Parser

Парсер читает файл построчно и разбивает его на секции.

Поддержанные форматы из текущей директории `data/`:

- `START_NODES` / `END_NODES` - CSV-секция узлов;
- `START_PORTS` / `END_PORTS` - CSV-секция портов;
- `SW_GUID=<id>` - секция дополнительной информации по switch;
- строки `key = value` внутри `SW_GUID` сохраняются в `nodes_info`.

Дополнительные неизвестные `START_*` секции пропускаются. Произвольная строка вне известных секций считается ошибкой формата.

## Topology Model

Минимальная модель топологии:

- `log` содержит результат одной загрузки;
- `node` принадлежит одному `log`;
- `node` содержит список `ports`;
- `nodes_info` хранит дополнительные пары `key/value`.

Endpoint `/api/v1/topology/{log_id}` возвращает список узлов и их портов. Полноценный graph engine не реализован.

Варианты развития связей между узлами:

- MAC: связывать порты по таблицам соседства и MAC-адресам;
- LLDP/CDP: строить ребра по discovery-протоколам;
- uplink/downlink: определять направление связи по роли порта;
- VLAN: связывать узлы в пределах VLAN;
- subnet: группировать интерфейсы по IP-подсетям.

## База данных

Таблицы:

- `logs`: метаданные загрузки, статус, счетчики, ошибка;
- `nodes`: сетевые узлы;
- `ports`: порты узлов;
- `nodes_info`: дополнительная информация по узлам;
- `schema_migrations`: служебная таблица примененных миграций.

Миграция находится в `migrations/001_init.sql`.

## Пример логов

Файлы лежат в `data/`:

```text
data/ibdiagnet2.db_csv
data/ibdiagnet2.sharp_an_info
```

Пример секции:

```text
START_NODES
NodeDesc,NumPorts,NodeType,ClassVersion,BaseVersion,SystemImageGUID,NodeGUID,PortGUID
"SWITCH_1",65,2,1,1,0xswitch1,0xswitch1,0xswitch1
END_NODES
```
