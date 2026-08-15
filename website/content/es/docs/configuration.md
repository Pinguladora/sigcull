---
title: "Configuración"
description: "La referencia completa de la política de sigcull: confianza, autoridades, coincidencia del committer y exenciones."
weight: 30
tags: ["policy"]
---

# Configuración

sigcull lee un único archivo YAML. Los secretos nunca se incluyen en línea. Se
leen de las variables de entorno nombradas en el archivo, o se montan como
archivos.

## Secretos

Se necesitan tres valores en tiempo de ejecución:

- `GITHUB_WEBHOOK_SECRET`: el secreto HMAC del webhook.
- `GITHUB_APP_ID`: el ID numérico de la App.
- `GITHUB_APP_PRIVATE_KEY`: el PEM de la clave privada de la App.

La clave privada puede montarse en su lugar como archivo con
`githubApp.privateKeyFile`, lo que evita las plataformas de contenedores que
estropean los saltos de línea en un valor de entorno multilínea.

```yaml
githubApp:
  appIDEnv: GITHUB_APP_ID
  privateKeyEnv: GITHUB_APP_PRIVATE_KEY
  # privateKeyFile: /etc/sigcull/app-key.pem   # tiene prioridad cuando se define
```

## Confianza

`trust.kind` selecciona de dónde vienen las raíces de verificación: `public`,
`tuf` o `static`. Cambiar entre ellas es solo configuración. Consulta
[Fuentes de confianza]({{< relref "trust-sources" >}}).

## Autoridades

`verify.authorities` es la lista ordenada de fuentes de firma aceptadas. Un
commit pasa si alguna autoridad lo acepta. Cada entrada define exactamente un
tipo. Consulta [Autoridades]({{< relref "authorities" >}}) para la lista completa.

## Coincidencia del committer

`verify.matchCommitter` exige además que el committer de git sea igual a la
identidad de firma: un subject de correo contra el correo del committer, un
subject de tipo URI contra el nombre del committer, o el correo de UID de una
clave GPG. Cierra el hueco en el que un firmante permitido hace un commit bajo el
nombre de otra persona. No se aplica a `githubVerified`, que no tiene identidad
de firmante que comparar.

## Enmascarado de identidad

`verify.showFullIdentity` es false por defecto, de modo que las direcciones de
correo del firmante se enmascaran en la salida del Check Run
(`a***@example.com`), ya que un Check Run en un repositorio público es
visible públicamente. La identidad completa siempre aparece en los registros de
la app.

## Exenciones

```yaml
exempt:
  skipMergeCommits: false
```

> [!CAUTION]
> `skipMergeCommits` exime cualquier commit con más de un padre, algo que quien
> hace el push puede fabricar. Mantenlo desactivado a menos que sepas que tus
> fusiones son seguras. Las fusiones del propio GitHub se gestionan mejor con una
> autoridad `githubVerified`, que las acepta en lugar de omitirlas.
