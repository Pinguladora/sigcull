---
title: "Registro de cambios"
description: "Cambios destacados en sigcull."
---

# Registro de cambios

Cambios destacados en sigcull. El proyecto sigue los commits convencionales, de
modo que las versiones se generan a partir del historial de commits.

## Sin publicar

- Se aceptan todos los formatos de firma de commits de git mediante autoridades
  combinables: Sigstore keyless, OpenPGP, SSH y X.509, además de la verificación
  propia de GitHub.
- La verificación se ejecuta por completo en proceso. Los commits se obtienen en
  memoria y se verifican con librerías, sin necesidad de un binario de git ni de
  gitsign.
- La clave privada de la App puede montarse como archivo, no solo leerse desde
  una variable de entorno.
- Los registros son JSON estructurado en stdout, con un `LOG_LEVEL` configurable.
