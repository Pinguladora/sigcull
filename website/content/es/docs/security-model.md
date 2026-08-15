---
title: "Modelo de seguridad"
description: "Verificación con fallo seguro, exenciones a prueba de suplantación y los límites de confianza."
weight: 60
tags: ["security"]
---

# Modelo de seguridad

sigcull es un control de acceso. Su trabajo es decidir si un commit falsificado o
sin firmar puede fusionarse, así que cada decisión está diseñada para fallar de
forma segura.

## Fallo seguro

Cualquier ambigüedad es un fallo. Un commit sin firmar, una firma inválida
criptográficamente, una firma válida cuya identidad no está en la lista de
permitidos, o una firma cuya identidad no puede leerse, todas producen un Check
Run fallido. Un commit se acepta solo cuando una autoridad lo verifica de forma
positiva.

## Suplantación del committer

El nombre y el correo del committer en un objeto commit los controla quien
ataca. sigcull nunca confía en ellos por sí solos para una decisión de seguridad.

- `githubVerified` confirma a través de la API de GitHub que el propio GitHub
  firmó el commit. Solo GitHub puede producir una firma que verifique para su
  propia identidad, así que quien hace el push no puede falsificarla.
- `matchCommitter` vincula al committer con la identidad de firma verificada, de
  modo que un firmante permitido no pueda hacer un commit bajo el nombre de otra
  persona.

## Exposición de identidad

Un Check Run en un repositorio público es visible públicamente. Las direcciones
de correo del firmante se enmascaran por defecto en la salida del Check Run,
mientras que la identidad completa permanece en los registros de la app. Define
`verify.showFullIdentity` como true en repositorios privados donde la exposición
no sea una preocupación.

## Autenticidad del webhook

Cada entrega se rechaza a menos que su HMAC `X-Hub-Signature-256` verifique
contra el secreto configurado. Nada aguas abajo se ejecuta ante una firma
incorrecta o ausente.

> [!WARNING]
> Un commit firmado con un correo de dominio propio es tan fiable como el control
> de ese dominio. Prefiere identidades ligadas a un issuer OIDC o a una clave que
> gestiones tú.
