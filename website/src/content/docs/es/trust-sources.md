---
title: Fuentes de confianza
description: La instancia public good, tu propio TUF o una raíz estática sin conexión.
---

`trust.kind` decide de dónde vienen las raíces de confianza de Sigstore. Las tres
se ejecutan por completo en memoria, sin almacenar nada en disco.

## public

La instancia *public good* de Sigstore. sigcull incorpora un `trusted_root.json`
empaquetado dentro del binario, sin necesidad de descargarlo.

```yaml
trust:
  kind: public
```

## static

Un archivo `trusted_root.json` local, leído una sola vez al arrancar. Totalmente
sin conexión, para instancias aisladas de la red.

```yaml
trust:
  kind: static
  trustRoot: /etc/sigcull/trusted_root.json
```

## tuf

Tu propio TUF. sigcull construye un cliente TUF integrado en el binario, contra un
*mirror* propio, usando un `root.json` como referencia.

```yaml
trust:
  kind: tuf
  tufMirror: https://tuf.internal.example.eu
  tufRoot: /etc/sigcull/root.json
```

:::note
La lista de identidades permitidas es agnóstica al emisor. El conjunto de emisores
desde los que puedes producir firmas es el que confíe el Fulcio de destino, así que
una instancia privada puede emitir identidades que la instancia pública nunca
aceptaría.
:::
