# Colombia Ecosystems Engine

Motor de juego por turnos para escenarios ambientales de Colombia, implementado en Go 1.22. El Amazonas es el primer escenario jugable.

## Incluye

- 30 cartas configurables basadas en el PDF del MVP.
- Cuatro sectores y tres recursos.
- Turnos, flechas, produccion y deforestacion.
- Doce eventos con tipping points, azar y mal gobierno.
- Tres rutas de victoria y tres condiciones de derrota.
- Guardado y carga de partidas JSON.
- CLI interactiva para probar una partida completa.

## Ejecutar

```bash
go run ./cmd/amazonas new --seed 42
```

Para cargar una partida:

```bash
go run ./cmd/amazonas load saves/partida.json
```

Dentro de la consola:

```text
status
hand
play <card-id>
events
resolve <event-id>
discard <card-id>
end
save saves/partida.json
help
quit
```

## Compilar

```bash
go build -o bin/amazonas ./cmd/amazonas
./bin/amazonas new --seed 42
```

## Verificar

```bash
make fmt
make vet
make test
make race
```

El `Makefile` usa el enlazador del sistema en macOS para mantener compatibilidad con Go 1.22.5 en versiones recientes del sistema operativo.

La suite incluye:

- Pruebas unitarias de cartas, requisitos, flechas, sectores, eventos, tipping points, victorias y derrotas.
- Validacion automatica del catalogo de 30 cartas y sus referencias.
- Round-trip de persistencia JSON.
- Integracion de caja negra usando las APIs publicas del motor.
- Integracion del binario real: iniciar, avanzar ronda, guardar y cargar.

Para medir cobertura:

```bash
go test -coverprofile=/tmp/amazonas-coverage.out ./...
go tool cover -func=/tmp/amazonas-coverage.out
```

## Arquitectura

- `domain`: contratos y estado serializable.
- `content`: carga y validacion de configuraciones.
- `engine`: reductor determinista y reglas.
- `persistence`: guardado atomico y carga validada.
- `cmd/amazonas`: consola interactiva.
- `assets`: contenido del escenario Amazonas.

El frontend futuro debe enviar comandos al motor y renderizar el `GameState` devuelto. Las reglas no deben duplicarse en la interfaz.

## API HTTP

Configura un secreto local y ejecuta:

```bash
export JWT_SECRET="replace-this-with-at-least-32-random-bytes"
make build-api
./bin/api
```

Recursos locales:

- API: `http://localhost:8080`
- Swagger UI: `http://localhost:8080/docs/`
- OpenAPI: `http://localhost:8080/openapi.yaml`
- Readiness: `http://localhost:8080/health/ready`

Crear una sesion de invitado:

```bash
curl -i -c /tmp/amazonas-cookies.txt \
  -X POST http://localhost:8080/api/v1/sessions/guest
```

La respuesta contiene `accessToken`. Usalo para crear una partida:

```bash
curl -X POST http://localhost:8080/api/v1/games \
  -H "Authorization: Bearer ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"seed":42}'
```

Aplicar un comando:

```bash
curl -X POST http://localhost:8080/api/v1/games/GAME_ID/commands \
  -H "Authorization: Bearer ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"type":"end_turn","expectedVersion":1}'
```

La API devuelve una vista publica: no expone la semilla interna, el orden del mazo, cooldowns ni eventos en cola.
