import type { APIRoute, GetStaticPaths } from "astro";
import { getCollection, type CollectionEntry } from "astro:content";
import { isArticlePage } from "../../lib/doc-pages";
import { markdownToTypst, typstString, escapeText, type Lang } from "../../lib/typst";

// Typst sources for the docs PDFs, written to dist/typst/<path>.typ. The site:pdf
// task compiles each one with Typst to a PDF/A-3a + PDF/UA-1 file:
//
//   - typst/sigcull-docs.typ, typst/es/sigcull-docs.typ: the full handbook per
//     language (cover, then the guide pages in ORDER, then every ADR), and
//   - typst/<id>.typ: one per article page (see isArticlePage), the target of the
//     page's Download-PDF action at /<id>.pdf.
//
// These are build intermediates, not site content: site:deploy removes
// dist/typst before publishing.

type Entry = CollectionEntry<"docs">;

const ORDER = [
  "getting-started",
  "authorities",
  "configuration",
  "trust-sources",
  "deployment",
  "security-model",
];

const HANDBOOK: Record<Lang, { title: string; tagline: string }> = {
  en: {
    title: "sigcull documentation",
    tagline:
      "A GitHub App that verifies the signature on every commit and turns it into a required merge gate.",
  },
  es: {
    title: "Documentación de sigcull",
    tagline:
      "Una GitHub App que verifica la firma de cada commit y la convierte en una comprobación obligatoria para poder fusionar.",
  },
};

const TEMPLATE = '#import "/pdf/template.typ": doc, cover, article, aside';

const langOf = (entry: Entry): Lang => (entry.id.startsWith("es/") ? "es" : "en");

function articleTypst(entry: Entry): string {
  const lang = langOf(entry);
  const body = markdownToTypst(entry.body ?? "", {
    lang,
    pageUrl: new URL(`/${entry.id}/`, import.meta.env.SITE).href,
  });
  return `#article(title: [${escapeText(entry.data.title)}], lang: ${typstString(lang)})[\n${body}\n];`;
}

function pageTypst(entry: Entry): string {
  return [
    TEMPLATE,
    `#show: doc.with(title: ${typstString(`${entry.data.title} · sigcull`)}, lang: ${typstString(langOf(entry))})`,
    articleTypst(entry),
  ].join("\n\n");
}

function handbookTypst(lang: Lang, docs: Entry[]): string {
  const prefix = lang === "es" ? "es/" : "";
  const main = ORDER.map((slug) => docs.find((e) => e.id === `${prefix}${slug}`)).filter(
    (e): e is Entry => e !== undefined,
  );
  // The ADRs exist only in English, so both handbooks carry the English ones.
  const adr = docs.filter((e) => /^adr\/\d/.test(e.id)).sort((a, b) => a.id.localeCompare(b.id));
  const { title, tagline } = HANDBOOK[lang];
  return [
    TEMPLATE,
    `#show: doc.with(title: ${typstString(title)}, lang: ${typstString(lang)})`,
    `#cover[${escapeText(tagline)}];`,
    ...[...main, ...adr].map(articleTypst),
  ].join("\n\n");
}

export const getStaticPaths: GetStaticPaths = async () => {
  const docs = await getCollection("docs");
  return [
    { params: { path: "sigcull-docs" }, props: { source: handbookTypst("en", docs) } },
    { params: { path: "es/sigcull-docs" }, props: { source: handbookTypst("es", docs) } },
    ...docs
      .filter(isArticlePage)
      .map((entry) => ({ params: { path: entry.id }, props: { source: pageTypst(entry) } })),
  ];
};

export const GET: APIRoute = ({ props }) =>
  new Response((props as { source: string }).source, {
    headers: { "Content-Type": "text/plain; charset=utf-8" },
  });
