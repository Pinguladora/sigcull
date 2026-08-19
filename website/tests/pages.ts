// Representative routes covering every page template, in both languages. The
// a11y sweep runs all of these; the visual sweep uses the smaller VISUAL subset.
// Starlight has a built-in search overlay (no /search/ page) and no taxonomy
// (no /tags/ page), so those Hugo routes are gone.
export const ROUTES = [
  { path: "/", name: "home" },
  { path: "/overview/", name: "overview" },
  { path: "/getting-started/", name: "getting-started" },
  { path: "/authorities/", name: "authorities" },
  { path: "/configuration/", name: "configuration" },
  { path: "/trust-sources/", name: "trust-sources" },
  { path: "/deployment/", name: "deployment" },
  { path: "/security-model/", name: "security-model" },
  { path: "/adr/", name: "adr-landing" },
  { path: "/adr/0001-verifier-single-call-parse/", name: "adr-0001" },
  { path: "/changelog/", name: "changelog" },
  // Spanish
  { path: "/es/", name: "es-home" },
  { path: "/es/overview/", name: "es-overview" },
  { path: "/es/authorities/", name: "es-authorities" },
  { path: "/es/security-model/", name: "es-security-model" },
  { path: "/es/changelog/", name: "es-changelog" },
] as const;

// Smaller set for visual regression: one of each distinct layout, both themes.
export const VISUAL_ROUTES = ROUTES.filter((r) =>
  ["home", "overview", "authorities", "adr-0001", "es-overview"].includes(r.name),
);

export const THEMES = ["light", "dark"] as const;
