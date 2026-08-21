# GARFEX Costos Unitarios

Core hexagonal (dominio + casos de uso + adaptador PostgreSQL) del Resource Master y Supplier Master, sin ninguna capa de interfaz (TUI, CLI, Web, API, MCP o Agentes) — esas capas viven fuera de este repositorio.

## Arquitectura

- [Límite de capacidades MCP](docs/architecture/mcp-capability-boundary.md)
- [Harness de runtime de agentes](docs/architecture/agent-runtime-harness.md)

## Repositorios y remotos Git

El código del producto usa el repositorio público como fuente canónica. El
repositorio privado del workspace sólo almacena artefactos locales que no
pertenecen al producto.

| Remoto | Responsabilidad |
| --- | --- |
| `origin` | Repositorio público canónico. Aloja `main`, ramas de producto, pull requests, CI y releases. |
| `workspace` | Repositorio privado opcional para artefactos del entorno de trabajo. No es la base de ramas de producto. |

La rama local `main` debe seguir `origin/main`. El flujo normal comienza desde
esa referencia:

```powershell
git switch main
git pull --ff-only origin main
git switch -c <tipo>/<descripcion>
```

Verificá la configuración después de clonar o modificar remotos:

```powershell
git remote -v
git branch -vv
```

Los archivos privados del workspace no deben incorporarse a ramas destinadas
al repositorio público.

## Requisitos

- Go 1.26.5 (declarado en `go.mod`).
- Docker Engine con Docker Compose v2 para PostgreSQL y la herramienta de migración.

## Configuración

Copiá `.env.example` a `.env` y reemplazá cada valor vacío o `CHANGE_ME`; `.env` está ignorado por Git y Compose lo carga automáticamente.

| Grupo | Variables | Uso |
| --- | --- | --- |
| Runtime | `GARFEX_DB_HOST`, `GARFEX_DB_PORT`, `GARFEX_DB_NAME`, `GARFEX_DB_USER`, `GARFEX_DB_PASSWORD`, `GARFEX_DB_SSLMODE` | Conexión a PostgreSQL para el adaptador de persistencia y los tests de integración. |
| Runtime | `GARFEX_LOG_LEVEL` | Opcional: `debug`, `info`, `warn` o `error`; omitir equivale a `info`. |
| Bootstrap Compose | `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` | Crea el cluster PostgreSQL. |
| Roles Compose | `GARFEX_ADMIN_PASSWORD`, `GARFEX_APP_PASSWORD` | Contraseñas de los roles de migración y runtime. |

## PostgreSQL local

Con `.env` completo, levantá y verificá la base:

```powershell
docker compose up -d --wait db
docker compose ps
docker compose restart db
```

El único servicio es `db`, publicado en `127.0.0.1:5432`, con el volumen nombrado `garfex_pgdata`; los datos persisten al reiniciar el servicio. El bootstrap crea `garfex_admin` (dueño de `public`, para migraciones) y `garfex_app` (runtime, solo `USAGE` en `public`); ambos son `NOSUPERUSER`, sin `CREATEDB` ni `CREATEROLE`. `POSTGRES_USER` es solo bootstrap y no debe usarse por la aplicación.

Para borrar intencionalmente la base local y volver a ejecutar los scripts de inicialización, usá `docker compose down -v`. Ese comando destruye `garfex_pgdata`.

## Migraciones

Las migraciones versionadas están en `migrations/`. La imagen fijada para ejecutarlas se puede verificar con:

```powershell
docker run --rm migrate/migrate:v4.18.2 -version
```

Ejecutalas con el DSN de `garfex_admin`; no uses `garfex_app` para administrar el esquema.

## Verificaciones

Los equivalentes locales de los controles no constructivos de CI son:

```powershell
gofmt -l .
go vet ./...
golangci-lint run ./...
go test ./... -count=1
docker compose config -q
```

`golangci-lint` requiere una versión compatible con Go 1.26.5 (probado con v2.12.2); instalala con `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2`. Su configuración vive en `.golangci.yml`.

CI ejecuta además `go test ./... -race -count=1` y `go build ./...`.
