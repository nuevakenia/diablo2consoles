# ADRs — las decisiones que no se re-discuten en cada tarjeta

Un ADR (*Architecture Decision Record*) es una decisión **tomada**, con tres cosas: el **contexto medido** (los
números que la obligan), las **alternativas descartadas con su número**, y las **consecuencias** (la disciplina
que implica y lo que queda abierto). Se lee **antes** de tocar la parte del árbol que cubre: para eso está.

## Los que hay

| # | decisión | estado | regla ejecutable |
|---|---|---|---|
| [0001](0001-assets-fuera-del-heap-de-go.md) | Los assets grandes del camino de consola viven **fuera del montón de Go**, con liberación explícita, y el motor no se toca: todo lo de consola vive detrás de la interfaz `Source` | aceptado (31/10/2026, 1b/2a/3a) | `scripts/console/checar-assets-por-la-source.sh` |

## Cómo se escribe el siguiente

- La numeración es **correlativa y no se reusa**: el próximo es **0002**.
- Archivo: `docs/adr/000N-<slug-en-espanol>.md`. En español de Chile, como todo el repo.
- **Si la decisión se puede vigilar sola, el ADR nombra su regla ejecutable** (un chequeo que se pone rojo
  cuando alguien la deshace) y, si hace falta, su mutador. Una decisión que sólo vive en el documento se pierde
  en la tercera tarjeta: el que llega nuevo copia lo que encuentra al lado.
- El contexto no se escribe de memoria: cada afirmación del contexto lleva su evidencia (`console/dc/evidencia/…`)
  y su número. "Funciona" sin número es un verde mentiroso.
- Si una decisión se revisa, se escribe un ADR **nuevo** que la reemplaza y el viejo queda con
  `Estado: reemplazado por 000N`. No se borra: la historia es parte del registro.
