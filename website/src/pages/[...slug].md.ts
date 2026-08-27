import type { APIRoute, GetStaticPaths } from "astro";
import { getCollection } from "astro:content";
import { isArticlePage } from "../lib/doc-pages";

// Raw-markdown view of each doc page, served at `/<slug>.md`. Powers the page's
// "Copy as Markdown" / "View as Markdown" actions and the LLM links. Splash and
// changelog pages are skipped (see isArticlePage): their source is JSX, not clean
// markdown.
export const getStaticPaths: GetStaticPaths = async () => {
  const docs = await getCollection("docs");
  return docs
    .filter(isArticlePage)
    .map((entry) => ({ params: { slug: entry.id }, props: { entry } }));
};

export const GET: APIRoute = ({ props }) => {
  const { entry } = props as { entry: { data: { title: string }; body?: string } };
  const md = `# ${entry.data.title}\n\n${entry.body ?? ""}`;
  return new Response(md, {
    headers: { "Content-Type": "text/markdown; charset=utf-8" },
  });
};
