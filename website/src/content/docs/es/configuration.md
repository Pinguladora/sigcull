---
title: Configuración
description: "La referencia completa de la política de sigcull: confianza, autoridades, coincidencia del committer y exenciones."
---

sigcull lee un único archivo YAML para su configuración. Los secretos nunca se
escriben en línea: se leen de las variables de entorno nombradas en el archivo, o
se montan como archivos.

## Secretos

Se necesitan tres valores en tiempo de ejecución:

- `GITHUB_WEBHOOK_SECRET`: el secreto HMAC del *webhook*.
- `GITHUB_APP_ID`: el ID de la App.
- `GITHUB_APP_PRIVATE_KEY`: el PEM de la clave privada de la App.

En su lugar, la clave privada puede montarse como archivo con
`githubApp.privateKeyFile`, útil cuando la plataforma no maneja bien los saltos de
línea de una variable de entorno multilínea.

```yaml
githubApp:
  appIDEnv: GITHUB_APP_ID
  privateKeyEnv: GITHUB_APP_PRIVATE_KEY
  # privateKeyFile: /etc/sigcull/app-key.pem   # tiene prioridad sobre privateKeyEnv
```

## Confianza

`trust.kind` selecciona de dónde proceden las raíces de verificación: `public`,
`tuf` o `static`. Consulta [Fuentes de confianza](/es/trust-sources/).

## Autoridades

`verify.authorities` es la lista ordenada de fuentes de firma aceptadas. Un
*commit* pasa si alguna autoridad lo acepta. Cada entrada define exactamente un
tipo. Consulta [Autoridades](/es/authorities/) para la lista completa.

## Coincidencia del *committer*

`verify.matchCommitter` exige además que el *committer* de git coincida con la
identidad de firma: un *subject* de correo frente al correo del *committer*, un
*subject* de tipo URI frente al nombre del *committer*, o el correo de UID de una
clave GPG. Cierra el hueco por el que un firmante permitido podría hacer un
*commit* en nombre de otra persona. No se aplica a `githubVerified`, que no tiene
una identidad de firmante con la que comparar.

## Enmascaramiento de identidad

`verify.showFullIdentity` es `false` por defecto, de modo que las direcciones de
correo del firmante se enmascaran en la salida del *Check Run* (`a***@example.com`),
ya que un *Check Run* en un repositorio público es visible públicamente.

:::caution
La identidad completa siempre aparece en los registros de la aplicación.
:::

## Exenciones

```yaml
exempt:
  skipMergeCommits: false
```

:::danger
`skipMergeCommits` exime cualquier *commit* con más de un padre, algo que puede
fabricar quien hace *push*. Mantenlo desactivado a menos que sepas que tus
*merges* son seguros. Los *merges* del propio GitHub se gestionan mejor con una
autoridad `githubVerified`, que los acepta en lugar de omitirlos.
:::
