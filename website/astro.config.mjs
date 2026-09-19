// @ts-check
import { defineConfig, fontProviders } from "astro/config";
import starlight from "@astrojs/starlight";
import starlightLlmsTxt from "starlight-llms-txt";

// Expressive Code renders each code block as a horizontally scrollable <pre>
// with no tabindex, so a keyboard-only user cannot scroll a wide block
// (axe scrollable-region-focusable, WCAG 2.1.1). Give every code <pre>
// tabindex="0" so it is reachable and scrollable from the keyboard.
function codeBlockKeyboardScroll() {
  const setTabindex = (node) => {
    if (node.type === "element" && node.tagName === "pre") {
      node.properties = node.properties || {};
      if (node.properties.tabIndex === undefined) node.properties.tabIndex = 0;
    }
    for (const child of node.children || []) setTabindex(child);
  };
  return {
    name: "code-block-keyboard-scroll",
    hooks: {
      postprocessRenderedBlock: ({ renderData }) => setTabindex(renderData.blockAst),
    },
  };
}

// sigcull docs. Starlight with en (root) and es locales. The sidebar lists
// pages by slug so each entry localizes to the current language automatically.
export default defineConfig({
  site: "https://sigcull.dev",
  // Responsive images: emit a populated srcset + sizes for content images
  // (multi-width responsive <img>), instead of a lone empty srcset.
  image: { layout: "constrained" },
  // Self-hosted brand font for the header wordmark. Astro downloads it at build
  // and serves it from our own origin (vendored, no runtime CDN), so it adds no
  // dependency. Exposed as var(--font-brand); rendered via <Font> in Head.astro.
  // Self-hosted brand font for the header wordmark. Astro downloads it at build
  // and serves it from our own origin (vendored, no runtime CDN), so it adds no
  // dependency. Climate Crisis is an eroded display face (its glyphs melt with a
  // YEAR axis), a fitting nod to poles and ice. Exposed as var(--font-brand);
  // rendered via <Font> in Head.astro.
  fonts: [
    {
      provider: fontProviders.google(),
      name: "Climate Crisis",
      cssVariable: "--font-brand",
      weights: [400],
      subsets: ["latin"],
      fallbacks: ["sans-serif"],
    },
  ],
  integrations: [
    starlight({
      title: "sigcull",
      description:
        "A GitHub App that verifies the signature on every commit and turns it into a required merge gate.",
      favicon: "/favicon.svg",
      components: {
        // Header wordmark in the brand font (see SiteTitle.astro); Head.astro
        // injects the <Font> tags; TableOfContents.astro adds a scroll-progress
        // line to the "On this page" nav.
        SiteTitle: "./src/components/SiteTitle.astro",
        Head: "./src/components/Head.astro",
        TableOfContents: "./src/components/TableOfContents.astro",
        PageTitle: "./src/components/PageTitle.astro",
        // Vendored copy of Starlight's Search that loads Pagefind on first intent
        // (hover/focus/open) instead of eagerly on idle, so visitors who never
        // search download nothing.
        Search: "./src/components/Search.astro",
      },
      defaultLocale: "root",
      locales: {
        root: { label: "English", lang: "en" },
        es: { label: "Español", lang: "es" },
      },
      social: [{ icon: "github", label: "GitHub", href: "https://github.com/Pinguladora/sigcull" }],
      editLink: {
        baseUrl: "https://github.com/Pinguladora/sigcull/edit/main/website/",
      },
      customCss: ["./src/styles/custom.css"],
      expressiveCode: {
        plugins: [codeBlockKeyboardScroll()],
        // Starlight localizes each code block's UI text (the terminal-window
        // label, the copy button) per block through getBlockLocale, but in this
        // Astro/Starlight pairing its detection resolves every block to the
        // default language, so Spanish pages showed English labels. Derive the
        // locale straight from the source path (src/content/docs/es/... is es) so
        // each block picks up the right translation Starlight already registers.
        getBlockLocale: ({ file }) => {
          const match = file.path.replace(/\\/g, "/").match(/\/content\/docs\/([^/]+)\//);
          return match && match[1] === "es" ? "es" : "en";
        },
      },
      sidebar: [
        { slug: "overview" },
        { slug: "getting-started" },
        { slug: "authorities" },
        { slug: "configuration" },
        { slug: "trust-sources" },
        { slug: "deployment" },
        { slug: "security-model" },
        { label: "Decision records", items: [{ autogenerate: { directory: "adr" } }] },
        { slug: "changelog" },
        { slug: "accessibility" },
      ],
      plugins: [starlightLlmsTxt()],
    }),
  ],
});
