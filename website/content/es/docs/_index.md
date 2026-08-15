---
title: "Documentación"
description: "Despliega sigcull, configura autoridades de firma y conviértelo en un control obligatorio."
weight: 1
bookFlatSection: false
---

# Documentación

sigcull es una GitHub App que verifica la firma de cada commit y convierte el
resultado en un Check Run obligatorio, de modo que los commits sin firmar o no
autorizados no puedan fusionarse en una rama protegida. Funciona en repositorios
privados y contra cualquier instancia de Sigstore.

![sigcull comprueba cada commit del rango en cinco etapas: el evento de GitHub, el webhook verificado con HMAC, una descarga superficial, la verificación por commit con gitsign y las demás autoridades, y un Check Run que aprueba o falla con anotaciones.](images/pipeline-es.png "Cómo sigcull verifica un commit")

Empieza por [Primeros pasos]({{< relref "getting-started" >}}) para tu primer
commit firmado y, a continuación, consulta:

- [Autoridades]({{< relref "authorities" >}}) para los formatos de firma que acepta sigcull.
- [Configuración]({{< relref "configuration" >}}) para la referencia completa de la política.
- [Fuentes de confianza]({{< relref "trust-sources" >}}) para raíces public, TUF y estáticas.
- [Despliegue]({{< relref "deployment" >}}) para ejecutar la App.
- [Modelo de seguridad]({{< relref "security-model" >}}) para los límites de confianza.
