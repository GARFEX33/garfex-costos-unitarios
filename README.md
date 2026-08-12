# GARFEX Costos Unitarios

Base operativa de Fase 0: CLI Go con TUI Bubble Tea, PostgreSQL local y migraciones versionadas. No contiene esquema de negocio ni servicios de aplicación.

## Arquitectura

- [Límite de capacidades MCP](docs/architecture/mcp-capability-boundary.md)
- [Harness de runtime de agentes](docs/architecture/agent-runtime-harness.md)

## Requisitos

- Go 1.26.5 (declarado en `go.mod`).
- Docker Engine con Docker Compose v2 para PostgreSQL y la herramienta de migración.

## Configuración

Copiá `.env.example` a `.env` y reemplazá cada valor vacío o `CHANGE_ME`; `.env` está ignorado por Git y Compose lo carga automáticamente.

| Grupo | Variables | Uso |
| --- | --- | --- |
| Runtime | `GARFEX_DB_HOST`, `GARFEX_DB_PORT`, `GARFEX_DB_NAME`, `GARFEX_DB_USER`, `GARFEX_DB_PASSWORD`, `GARFEX_DB_SSLMODE` | Requeridas por `garfex config check`. |
| Runtime | `GARFEX_LOG_LEVEL` | Opcional: `debug`, `info`, `warn` o `error`; omitir equivale a `info`. |
| Bootstrap Compose | `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` | Crea el cluster PostgreSQL. |
| Roles Compose | `GARFEX_ADMIN_PASSWORD`, `GARFEX_APP_PASSWORD` | Contraseñas de los roles de migración y runtime. |

Para ejecutar la CLI desde PowerShell, definí las variables `GARFEX_*` en el proceso actual (la CLI no lee archivos `.env`) y ejecutá:

```powershell
go run ./cmd/garfex config check
```

El resumen de `config check` redacta la contraseña. Los subcomandos directos disponibles son `version` y `config check`.

## Menú interactivo

Ejecutá `go run ./cmd/garfex` sin argumentos para abrir el menú Bubble Tea. Como verificación manual, navegá con las flechas o `j`/`k`, abrí **Version**, **Config check** y **GARFEX status**, y después elegí **Exit**. `config check` usa las variables `GARFEX_*` del proceso actual; si faltan, el menú muestra el error de configuración sin exponer la contraseña.

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

CI ejecuta además `go test ./... -race -count=1`, `go build ./...`, construye una imagen Docker etiquetada, verifica `Config.User=65532:65532` y ejecuta `version`; no ejecutes esos builds localmente después de cambios.
