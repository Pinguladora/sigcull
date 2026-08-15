// Representative routes covering every page template, in both languages. The
// a11y sweep runs all of these; the visual sweep uses the smaller VISUAL subset.
export const ROUTES = [
  { path: "/", name: "home" },
  { path: "/docs/", name: "docs-landing" },
  { path: "/docs/getting-started/", name: "docs-getting-started" },
  { path: "/docs/authorities/", name: "docs-authorities" },
  { path: "/docs/configuration/", name: "docs-configuration" },
  { path: "/docs/trust-sources/", name: "docs-trust-sources" },
  { path: "/docs/deployment/", name: "docs-deployment" },
  { path: "/docs/security-model/", name: "docs-security-model" },
  { path: "/docs/adr/", name: "adr-landing" },
  { path: "/docs/adr/0001-verifier-single-call-parse/", name: "adr-0001" },
  { path: "/changelog/", name: "changelog" },
  { path: "/search/", name: "search" },
  { path: "/tags/", name: "tags" },
  // Spanish
  { path: "/es/", name: "es-home" },
  { path: "/es/docs/", name: "es-docs-landing" },
  { path: "/es/docs/authorities/", name: "es-docs-authorities" },
  { path: "/es/docs/security-model/", name: "es-docs-security-model" },
  { path: "/es/changelog/", name: "es-changelog" },
  { path: "/es/search/", name: "es-search" },
] as const;

// Smaller set for visual regression: one of each distinct layout, both themes.
export const VISUAL_ROUTES = ROUTES.filter((r) =>
  ["home", "docs-landing", "docs-authorities", "adr-0001", "search", "es-docs-landing"].includes(r.name),
);

export const THEMES = ["light", "dark"] as const;
