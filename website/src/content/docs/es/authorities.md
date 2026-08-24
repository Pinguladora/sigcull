---
title: Autoridades
description: Los formatos de firma que acepta sigcull, evaluados con semántica OR.
---

Una autoridad es una fuente aceptada de firma válida. Un *commit* pasa si
**alguna** de las autoridades configuradas lo acepta, así que puedes combinar los
formatos que use tu equipo. sigcull cubre todos los formatos de firma que git
puede producir, además de la verificación propia de GitHub.

:::note
Las autoridades se prueban en orden. Gana la primera que acepta un *commit*, y el
*Check Run* informa de cuál fue esa autoridad.
:::

## keyless

Firmas *keyless* de Sigstore producidas por gitsign. Cada firma se verifica frente
a una cadena de certificados de Fulcio y a una prueba de inclusión de Rekor, y
luego se compara la identidad del firmante con tu lista de identidades permitidas.

```yaml
- keyless:
    allow:
      - issuer: https://accounts.google.com
        san: you@example.com
      - issuer: https://token.actions.githubusercontent.com
        sanRegex: "^https://github.com/acme/.+/.github/workflows/.+@refs/heads/main$"
```

El `issuer` (emisor OIDC) se compara de forma exacta. El *subject* es `san`
(coincidencia exacta) o `sanRegex` (una expresión regular de Go, anclada
automáticamente).

## gpgKey

Firmas OpenPGP tradicionales, verificadas en el propio proceso frente a un llavero
de confianza. La clave de firma debe llevar uno de los correos de UID permitidos.

```yaml
- gpgKey:
    keyringPath: /etc/sigcull/trusted-keys.asc
    allowEmails:
      - dev@acme.com
```

## sshKey

Firmas SSH (SSHSIG), verificadas frente a un archivo `allowed_signers` de OpenSSH.
`allowPrincipals` restringe de forma opcional qué *principals* de ese archivo se
aceptan.

```yaml
- sshKey:
    allowedSignersPath: /etc/sigcull/allowed_signers
    allowPrincipals:
      - dev@acme.com
```

## x509

Firmas CMS separadas (S/MIME) cuyo certificado de firma encadena hasta una
autoridad de certificación privada.

```yaml
- x509:
    caBundlePath: /etc/sigcull/ca-bundle.pem
    allowEmails:
      - dev@acme.com
```

## githubVerified

*Commits* que el propio GitHub ha verificado criptográficamente, como los *merges*
web y de *squash*. Con `webFlowOnly`, solo califica la clave *web-flow* del propio
GitHub, no la clave del modo *vigilant* de un usuario que GitHub se limita a
verificar.

```yaml
- githubVerified:
    webFlowOnly: true
```

:::caution
`githubVerified` se apoya en la verificación del propio GitHub, confirmada a través
de la API, así que un *committer* suplantado no puede autoaceptarse. Consulta el
[modelo de seguridad](/es/security-model/).
:::
