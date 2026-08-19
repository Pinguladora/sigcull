---
title: Primeros pasos
description: Instala la GitHub App y protege tu primer commit.
---

sigcull se ejecuta como una GitHub App. Recibe un webhook en cada push y cada
pull request, obtiene los commits que introduce una rama, verifica cada uno
contra las autoridades que hayas configurado e informa del resultado como un
Check Run.

## Instala la App

Crea una GitHub App con permisos de lectura y escritura en Checks, lectura en
Contents y lectura en Metadata. Suscríbela a los eventos Push, Pull request y
Check run. Apunta el webhook a tu despliegue y define un secreto de webhook.

## Configura una autoridad

Un commit pasa si lo acepta cualquiera de las autoridades configuradas. La
política más sencilla es una única autoridad keyless que confía en una identidad
de Sigstore.

```yaml
verify:
  authorities:
    - keyless:
        allow:
          - issuer: https://accounts.google.com
            san: you@example.com
```

## Conviértelo en obligatorio

Añade `sigcull` como control de estado obligatorio en un conjunto de reglas del
repositorio o en una regla de protección de rama. A partir de ese momento, un
pull request no podrá fusionarse a menos que cada commit no exento cumpla la
política.
