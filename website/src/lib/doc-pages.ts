// Shared rule for which doc-collection entries are full article pages, meaning
// the ones that get a per-page `.md` endpoint, a per-page PDF, and the on-page
// actions. Two kinds of entry are excluded:
//
//   - the splash homepages, matched by `template: 'splash'` (Astro drops the
//     `/index` suffix from an entry id, so `es/index.mdx` has the id `es`, which
//     is why an id list like `['es/index']` silently missed it and leaked a stray
//     `es.pdf`), and
//   - the changelog, authored as MDX (JSX, not clean markdown), matched by id.
//
// One predicate keeps the per-page endpoints, the print route, and PageActions in
// agreement, so a page can never get a button that points at a missing file.
const NON_ARTICLE_IDS = new Set(["changelog", "es/changelog"]);

export function isArticlePage(entry: { id: string; data: { template?: string } }): boolean {
  return entry.data.template !== "splash" && !NON_ARTICLE_IDS.has(entry.id);
}
