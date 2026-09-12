---
title: Accesibilidad
description: Cómo se construye y se prueba la accesibilidad del sitio de documentación de sigcull, y cómo informar de un problema.
---

sigcull se compromete a que su documentación sea accesible para todo el mundo, sean cuales sean sus capacidades o la tecnología que utilice. La accesibilidad forma parte de cómo se construye el sitio, no es algo que se añada al final, y se mejora de forma continua.

## Nivel de conformidad

Las [Pautas de Accesibilidad para el Contenido Web (WCAG)](https://www.w3.org/WAI/standards-guidelines/wcag/) definen los requisitos para hacer el contenido web más accesible. Este sitio se construye y se prueba para cumplir el **nivel AA de las WCAG 2.2**.

Ninguna comprobación garantiza el cumplimiento de todos los criterios, así que describimos el sitio como sustancialmente conforme: cuando aparece un problema, lo corregimos. Si encuentras algo que no llega al nivel AA, te pedimos que nos lo cuentes (consulta [Contacto](#contacto)).

## Medidas que adoptamos

- La accesibilidad se tiene en cuenta durante el diseño y el desarrollo, no se añade al final.
- Cada cambio se comprueba con las pruebas de más abajo antes de publicarse.
- Agradecemos y atendemos los avisos de quienes leen la documentación.

## Cómo se prueba el sitio

Cada página del sitio ya construido se comprueba, en el tema claro y en el oscuro, con:

- **axe-core** ejecutado contra los conjuntos de reglas de nivel A y AA de las WCAG 2.0, 2.1 y 2.2 en cada página, lo que también cubre el uso de ARIA y la estructura de regiones de referencia.
- Manejo **solo con teclado**, de modo que cada control y cada bloque de código desplazable se pueda alcanzar y usar sin ratón, y sin trampas de teclado.
- **Movimiento reducido**: con `prefers-reduced-motion` activado, se desactivan las animaciones y el desplazamiento suave.
- **NVDA**, un lector de pantalla real controlado con guidepup, recorriendo los flujos de lectura y de anuncios.
- Auditorías de accesibilidad de **Lighthouse** en todo el sitio.
- Validación del HTML generado.

## Características de accesibilidad

- Un diseño adaptable que se reorganiza en una sola columna y sigue siendo usable con un zoom del 200 %.
- Un enlace para saltar directamente al contenido principal.
- HTML semántico con regiones de referencia (cabecera, navegación, contenido principal, contenido complementario y pie) y una estructura de encabezados correcta y ordenada en cada página.
- ARIA cuando el HTML nativo no basta, por ejemplo para marcar la página actual en la navegación.
- Manejo completo con teclado, incluidos los bloques de código enfocables y desplazables, con indicadores de foco visibles.
- Colores de texto e interfaz que cumplen las relaciones de contraste del nivel AA de las WCAG, tanto en el tema claro como en el oscuro, y el texto además cumple la relación más estricta del nivel AAA (WCAG 1.4.6).
- Se respeta la preferencia de reducir el movimiento (`prefers-reduced-motion`).
- El idioma de la página declarado en cada página, para que la tecnología de apoyo lea correctamente el contenido en inglés y en español.
- Alternativas textuales para las imágenes.
- Títulos de página descriptivos, un índice "En esta página" y el buscador, para encontrar el contenido de más de una forma.
- PDF descargables, etiquetados y validados como **PDF/UA-1** (accesible) y **PDF/A-3a** (de archivo).

## Especificaciones técnicas

La accesibilidad se apoya en HTML, WAI-ARIA, CSS y JavaScript, y en que tu navegador y la tecnología de apoyo que uses los admitan. El sitio se genera de forma estática, así que su contenido se puede leer sin JavaScript.

## Limitaciones conocidas

No tenemos constancia de criterios de nivel AA sin cumplir. Las herramientas automáticas solo cubren una parte de las WCAG, así que si encuentras un problema que nuestras pruebas no detectaron, nos gustaría saberlo.

## Contacto

Si te encuentras con un problema de accesibilidad en este sitio, abre una incidencia en [github.com/Pinguladora/sigcull/issues](https://github.com/Pinguladora/sigcull/issues). Indícanos la página, qué ha fallado y el navegador y la tecnología de apoyo que utilizabas. Intentaremos responder con prontitud.

## Sobre esta declaración

Esta declaración cubre el sitio de documentación de sigcull. Se revisó por última vez el 30 de agosto de 2026.
