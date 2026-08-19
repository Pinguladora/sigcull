---
title: Despliegue
description: Ejecuta sigcull como contenedor y conviértelo en un control de estado obligatorio.
---

sigcull es un único binario sin estado. No necesita base de datos, ni disco, ni
subprocesos, así que se distribuye como una imagen distroless pequeña y escala
horizontalmente.

## Ejecución

Proporciona los secretos como variables de entorno y apunta la App a tu
configuración.

```sh
export GITHUB_WEBHOOK_SECRET=...
export GITHUB_APP_ID=...
export GITHUB_APP_PRIVATE_KEY="$(cat app.private-key.pem)"
go run ./cmd/sigcull -config config.yaml
```

## Contenedor

```sh
docker build -f Containerfile -t sigcull .
docker run --rm -p 8080:8080 \
  -e GITHUB_WEBHOOK_SECRET -e GITHUB_APP_ID -e GITHUB_APP_PRIVATE_KEY \
  -v "$PWD/config.yaml:/etc/sigcull/config.yaml:ro" \
  sigcull
```

El endpoint del webhook es `/webhook` y hay un endpoint de liveness en
`/healthz`. Define `LOG_LEVEL` (`debug`, `info`, `warn`, `error`) para controlar
el detalle. Los registros son JSON estructurado en stdout.

:::note
El procesamiento es asíncrono: sigcull responde al webhook de inmediato y luego
verifica en segundo plano. Ejecútalo como una instancia siempre activa, no como
serverless scale-to-zero, para que la verificación en segundo plano siempre se
complete.
:::

## Conviértelo en obligatorio

Añade `sigcull` como control de estado obligatorio en un conjunto de reglas del
repositorio o en una regla de protección de rama. Los pull requests no podrán
fusionarse a menos que cada commit no exento cumpla la política.
