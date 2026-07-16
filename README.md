# Colombia Ecosystems Engine

Motor de juego por turnos para escenarios ambientales de Colombia, implementado en Go 1.22. El Amazonas es el primer escenario jugable.

## Dataset de referencia

Los escenarios y umbrales del juego están inspirados por datos reales de focos de calor satelitales de la Amazonia colombiana:

- **Link:** https://www.datos.gov.co/dataset/Puntos-de-calor-por-regi-n-Hist-rico-Escala-1-100-/4dyk-z4e2/about_data
- **Entidad:** SIATAC – Sistema de Información Ambiental Territorial de la Amazonia Colombiana
- **Cobertura:** 10 departamentos de la Amazonia y Orinoquía, desde enero 2017 hasta la fecha actual

## Repositorio frontend y modelo de predicción

La lógica de análisis de datos y el modelo predictivo de focos de calor (RandomForestRegressor por departamento) se encuentran en:

- **Repositorio:** https://github.com/devscastellanos/AmazonIA-front
- **Scripts:** `analytics/limpiar_datos.py` y `analytics/prediccion_datos.py`

## Incluye

- 30 cartas configurables basadas en el PDF del MVP.
- Cuatro sectores y tres recursos.
- Turnos, flechas, produccion y deforestacion.
- Doce eventos con tipping points, azar y mal gobierno.
- Tres rutas de victoria y tres condiciones de derrota.
- Tres dificultades configurables: facil, normal y dificil.
- Guardado y carga de partidas JSON.
- CLI interactiva para probar una partida completa.

## Ejecutar

```bash
go run ./cmd/amazonas new --seed 42 --difficulty easy
```

Para cargar una partida:

```bash
go run ./cmd/amazonas load saves/partida.json
```

Si `--difficulty` se omite, la partida comienza en dificultad `easy`.

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

Para ejecutar simulaciones reproducibles de balance:

```bash
make simulate
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
- `docs/api`: contrato OpenAPI y Swagger UI embebidos.

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

La coleccion importable de Postman esta en
`docs/postman/colombia-ecosystems-api.postman_collection.json`. Incluye un flujo
completo de sesion, catalogo, partida, comandos y limpieza; la variable
`baseUrl` puede ajustarse al puerto donde se ejecute la API. En entornos donde
el puerto 8080 este ocupado, use por ejemplo `http://localhost:8081`.

La referencia campo por campo para implementar un cliente web esta en
`docs/guia-integracion-frontend.md`.

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
  -d '{"seed":42,"difficulty":"easy"}'
```

En HTTP, omitir `difficulty` también crea la partida en modo `easy`. Para elegir
otro perfil, envía `"difficulty":"normal"` o `"difficulty":"hard"`.

## Presets de prueba

Para probar desenlaces en la ronda 2, inicia la API local con:

```bash
APP_ENV=development ENABLE_TEST_PRESETS=true \
HTTP_ADDR=127.0.0.1:8081 \
JWT_SECRET="local-development-secret-at-least-32-bytes" \
./bin/api
```

Al crear una partida, usa uno de estos valores en `testPreset`:

- `victory_restoration_round_2`
- `defeat_social_round_2`
- `defeat_environmental_round_2`
- `defeat_territorial_round_2`

Ejecuta `end_turn` dos veces usando la version devuelta por cada respuesta. Los
presets estan desactivados por defecto y la API no permite habilitarlos cuando
`APP_ENV=production`.

Aplicar un comando:

```bash
curl -X POST http://localhost:8080/api/v1/games/GAME_ID/commands \
  -H "Authorization: Bearer ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"type":"end_turn","expectedVersion":1}'
```

La API devuelve una vista publica: no expone la semilla interna, el orden del mazo, cooldowns ni eventos en cola.
