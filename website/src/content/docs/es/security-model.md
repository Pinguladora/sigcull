---
title: Modelo de seguridad
description: Verificación con fallo seguro, exenciones a prueba de suplantación y los límites de confianza.
---

sigcull es un control de acceso. Su trabajo es decidir si se puede hacer *merge* de
un *commit* falsificado o sin firmar, así que cada decisión está diseñada para
fallar de forma segura.

## Fallo seguro

Cualquier ambigüedad es un fallo. Un *commit* sin firmar, una firma inválida
criptográficamente, una firma válida cuya identidad no está en la lista de
permitidos, o una firma cuya identidad no puede leerse: todas producen un
*Check Run* fallido. Un *commit* se acepta solo cuando una autoridad lo verifica
de forma positiva.

## Suplantación del *committer*

El nombre y el correo del *committer* de un objeto *commit* los controla el
atacante, así que sigcull nunca confía en ellos por sí mismos.

- `githubVerified` confirma a través de la API de GitHub que el propio GitHub firmó
  el *commit*. Solo GitHub puede producir una firma que verifique para su propia
  identidad, así que quien hace *push* no puede falsificarla.
- `matchCommitter` vincula al *committer* con la identidad de firma verificada, de
  modo que un firmante permitido no pueda suplantar a otra persona.

## Exposición de identidad

Un *Check Run* en un repositorio público es visible públicamente. Las direcciones
de correo del firmante se enmascaran por defecto en la salida del *Check Run*,
mientras que la identidad completa permanece en los registros de la aplicación.
Este comportamiento se cambia poniendo `verify.showFullIdentity` en `true`.

## Autenticidad del *webhook*

Cada entrega se rechaza a menos que su HMAC `X-Hub-Signature-256` se verifique
frente al secreto configurado.

:::caution
Un *commit* firmado con un correo de un dominio propio es tan fiable como el
control que tengas de ese dominio. Emplea identidades ligadas a un emisor OIDC o a
una clave que gestiones tú.
:::
