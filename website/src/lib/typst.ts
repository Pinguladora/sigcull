// Markdown to Typst markup, for the docs PDFs (see src/pages/typst/ and
// pdf/template.typ). Each doc's markdown source is parsed to an mdast tree with
// the same syntax extensions Starlight enables (GFM and the :::aside directives)
// and emitted as Typst. Typst then compiles it straight to a PDF/A-3a + PDF/UA-1
// file, with no HTML step in between.
//
// Every element is emitted in Typst's function form (`#heading(level: 2)[…];`,
// `#raw("…");`) and ended with `;`, so whatever text follows can never extend
// the expression. Plain text is escaped character by character. Any markdown
// construct this module does not know throws, which fails the build instead of
// silently dropping content from the PDF.
import type { Nodes, Parents, PhrasingContent, Root, RootContent } from "mdast";
import { fromMarkdown } from "mdast-util-from-markdown";
import { directiveFromMarkdown } from "mdast-util-directive";
import { gfmFromMarkdown } from "mdast-util-gfm";
import { directive } from "micromark-extension-directive";
import { gfm } from "micromark-extension-gfm";

export type Lang = "en" | "es";

// Starlight's own aside labels for the two site languages.
const ASIDE_LABELS: Record<Lang, Record<string, string>> = {
  en: { note: "Note", tip: "Tip", caution: "Caution", danger: "Danger" },
  es: { note: "Nota", tip: "Consejo", caution: "Precaución", danger: "Peligro" },
};

export interface ConvertOptions {
  lang: Lang;
  // Absolute URL of the page on the site. Site-relative links (`/configuration/`,
  // `#trust`) resolve against it, so every link in the PDF points to a live page.
  pageUrl: string;
}

export function parseMarkdown(markdown: string): Root {
  return fromMarkdown(markdown, {
    extensions: [gfm(), directive()],
    mdastExtensions: [gfmFromMarkdown(), directiveFromMarkdown()],
  });
}

/** Convert a doc's markdown body (front matter already stripped) to Typst markup. */
export function markdownToTypst(markdown: string, options: ConvertOptions): string {
  const tree = parseMarkdown(markdown);
  const definitions = new Map<string, string>();
  collectDefinitions(tree, definitions);
  return new Emitter(options, definitions).blocks(tree.children);
}

/** A Typst string literal. */
export function typstString(value: string): string {
  return `"${value.replace(/[\\"]/g, "\\$&").replace(/\n/g, "\\n").replace(/\r/g, "\\r").replace(/\t/g, "\\t")}"`;
}

/**
 * Escape plain text for Typst markup.
 *
 * Astro renders the site with smartypants, which turns `---`, `--` and `...` into
 * an em dash, an en dash and an ellipsis. Those are applied here as characters
 * first, so the PDF reads like the page. Straight quotes are left unescaped:
 * Typst's smart quotes curl them just as smartypants does. Every other character
 * Typst markup treats as syntax (markup, comments, labels, references, math,
 * shorthands, list and heading markers, autolinked URLs) is backslash-escaped.
 */
export function escapeText(text: string): string {
  return text
    .replace(/\s*\n\s*/g, " ")
    .replace(/---/g, "—")
    .replace(/--/g, "–")
    .replace(/\.\.\./g, "…")
    .replace(/[\\/*_`$#<>@[\]~=+\-.]/g, "\\$&");
}

function collectDefinitions(node: Nodes, into: Map<string, string>): void {
  if (node.type === "definition") into.set(node.identifier, node.url);
  if ("children" in node) for (const child of node.children) collectDefinitions(child, into);
}

class Emitter {
  constructor(
    private readonly options: ConvertOptions,
    private readonly definitions: Map<string, string>,
  ) {}

  blocks(nodes: RootContent[]): string {
    return nodes
      .map((node) => this.block(node))
      .filter((out) => out !== "")
      .join("\n\n");
  }

  private block(node: RootContent): string {
    switch (node.type) {
      case "paragraph":
        return this.inline(node.children);
      case "heading":
        return `#heading(level: ${node.depth})[${this.inline(node.children)}];`;
      case "code": {
        const lang = node.lang ? `lang: ${typstString(node.lang)}, ` : "";
        return `#raw(block: true, ${lang}${typstString(node.value)});`;
      }
      case "blockquote":
        return `#quote(block: true)[${this.blocks(node.children)}];`;
      case "list": {
        const items = node.children.map((item) => `[${this.blocks(item.children)}]`).join(", ");
        const tight = `tight: ${node.spread ? "false" : "true"}`;
        return node.ordered
          ? `#enum(start: ${node.start ?? 1}, ${tight}, ${items});`
          : `#list(${tight}, ${items});`;
      }
      case "thematicBreak":
        return "#line(length: 100%);";
      case "table": {
        const [head, ...rows] = node.children;
        const cell = (c: { children: PhrasingContent[] }) => `[${this.inline(c.children)}]`;
        const header = head ? `table.header(${head.children.map(cell).join(", ")}), ` : "";
        const body = rows.flatMap((row) => row.children.map(cell)).join(", ");
        const columns = head?.children.length ?? 1;
        return `#table(columns: ${columns}, ${header}${body});`;
      }
      case "containerDirective":
        return this.aside(node);
      case "definition":
        // Consumed up front by collectDefinitions; renders nothing.
        return "";
      default:
        return unsupported(node);
    }
  }

  private aside(node: Extract<RootContent, { type: "containerDirective" }>): string {
    const labels = ASIDE_LABELS[this.options.lang];
    const fallback = labels[node.name];
    if (fallback === undefined) return unsupported(node, `:::${node.name}`);
    // A custom title (`:::note[Heads up]`) arrives as a first paragraph flagged
    // as the directive label.
    const [first, ...rest] = node.children;
    const custom = first?.type === "paragraph" && first.data?.directiveLabel === true;
    const label = custom ? this.inline(first.children) : escapeText(fallback);
    const body = this.blocks(custom ? rest : node.children);
    return `#aside(kind: ${typstString(node.name)}, label: [${label}])[${body}];`;
  }

  private inline(nodes: PhrasingContent[]): string {
    return nodes.map((node) => this.phrase(node)).join("");
  }

  private phrase(node: PhrasingContent): string {
    switch (node.type) {
      case "text":
        return escapeText(node.value);
      case "strong":
        return `#strong[${this.inline(node.children)}];`;
      case "emphasis":
        return `#emph[${this.inline(node.children)}];`;
      case "delete":
        return `#strike[${this.inline(node.children)}];`;
      case "inlineCode":
        return `#raw(${typstString(node.value)});`;
      case "break":
        return "#linebreak();";
      case "link":
        return this.link(node.url, node.children);
      case "linkReference": {
        const url = this.definitions.get(node.identifier);
        if (url === undefined) return unsupported(node, `undefined link reference [${node.label}]`);
        return this.link(url, node.children);
      }
      default:
        return unsupported(node);
    }
  }

  private link(url: string, children: PhrasingContent[]): string {
    const href = new URL(url, this.options.pageUrl).href;
    return `#link(${typstString(href)})[${this.inline(children)}];`;
  }
}

function unsupported(node: Nodes | Parents, what: string = node.type): never {
  const where = node.position ? ` at line ${node.position.start.line}` : "";
  throw new Error(
    `typst: unsupported markdown (${what})${where}. Extend src/lib/typst.ts so the PDF keeps this content.`,
  );
}
