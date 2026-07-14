# Balance de dificultades

## Objetivo

El motor ofrece tres perfiles configurables sin cambiar las cartas. `normal`
conserva los valores base del MVP, `easy` amplía el margen de recuperación y
`hard` exige más recursos y tolera menos crisis.

| Regla | Fácil | Normal | Difícil |
|---|---:|---:|---:|
| Recursos iniciales (dinero/personas/tierra) | 3/2/1 | 2/1/0 | 1/1/0 |
| Deforestación inicial | 250 | 340 | 500 |
| Eventos activos máximos | 2 | 3 | 4 |
| Multiplicador de eventos aleatorios | 0.75 | 1.00 | 1.25 |
| Intervalo de mal gobierno | 20 | 16 | 12 |
| Límite de presión social | 4 | 3 | 2 |
| Límite de fallos territoriales | 3 | 2 | 2 |
| Modificador de cartas para victoria | -1 | 0 | +1 |

Los requisitos de Personas y Tierra nunca bajan de cero. Las cartas necesarias
para una ruta siempre se limitan entre una y la cantidad disponible en la ruta.

## Simulador

`cmd/simulate` ejecuta partidas completas con semillas reproducibles. El agente
elige una ruta de victoria, conserva sus cartas y prerrequisitos, resuelve eventos
pagables, juega cartas válidas y descarta las menos útiles.

```bash
go run ./cmd/simulate --games 100 --max-rounds 80 --seed 1
```

El reporte incluye victorias, derrotas, partidas sin desenlace, rondas promedio,
rutas completadas y causas de derrota. Una corrida de referencia de 100 partidas
por perfil produjo 62% de victorias en fácil, 7% en normal y 1% en difícil.

Estos valores miden al agente heurístico, no a jugadores humanos. La diferencia
entre perfiles es válida, pero la baja frecuencia de Transición sostenible y
Bienestar social señala que esas rutas deben observarse en pruebas de juego. El
archivo JSON de dificultades permite ajustar el balance sin cambiar el motor.

## Reglas de derrota visibles

- Colapso ambiental: deforestación igual o superior a 3000, en todos los perfiles.
- Colapso social: presión social igual o superior al límite de la dificultad.
- Colapso territorial: fallos territoriales igual o superior al límite del perfil.

Recursos en cero no generan una derrota instantánea. Los eventos y fallos elevan
los contadores visibles, dando al jugador oportunidad de reaccionar.
