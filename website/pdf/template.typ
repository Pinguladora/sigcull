// Typst layout for the docs PDFs, the counterpart of the website's stylesheet.
// The Astro build turns each doc's markdown into a small .typ file under
// dist/typst/ (see src/pages/typst/[...path].typ.ts) that imports this template,
// and the site:pdf task compiles it with `--pdf-standard a-3a,ua-1`, so every PDF
// is both archival (PDF/A-3a) and accessible (PDF/UA-1) straight out of Typst.
//
// Keep it semantic: real headings, lists, quotes and raw blocks become the tagged
// structure tree, while the page footer and the decorative rules are artifacts.
// Only Typst's bundled fonts are used (the task passes --ignore-system-fonts), so
// the output does not depend on what the build machine has installed.

#let ink = rgb("#111111")
#let muted = rgb("#555555")
#let accent = rgb("#0a5b63")
#let code-bg = rgb("#f5f5f5")

// Whole-document setup: metadata, page, type, and the element styles.
#let doc(title: none, lang: "en", body) = {
  set document(title: title, author: "sigcull")
  set text(font: "Libertinus Serif", size: 11pt, lang: lang, fill: ink)
  set par(leading: 0.75em, spacing: 1.1em)
  set page(
    paper: "a4",
    margin: (x: 18mm, y: 20mm),
    footer: context align(center, text(
      size: 9pt,
      fill: muted,
      counter(page).display("1 / 1", both: true),
    )),
  )

  show heading: set block(above: 1.4em, below: 0.7em, sticky: true)
  show heading.where(level: 1): set text(size: 20pt)
  show heading.where(level: 2): set text(size: 15pt)
  show heading.where(level: 3): set text(size: 12.5pt)

  // Underline links, never color alone, so link text is distinguishable without
  // relying on color (WCAG 1.4.1 Use of Color).
  show link: it => underline(text(fill: accent, it))

  // One high-contrast ink for code: no syntax theme, so no token color can fall
  // below the WCAG AA contrast ratio on the light code background.
  set raw(theme: none)
  show raw: set text(font: "DejaVu Sans Mono", size: 9.5pt)
  show raw.where(block: false): box.with(fill: code-bg, inset: (x: 2pt), outset: (y: 2pt))
  show raw.where(block: true): block.with(
    width: 100%,
    fill: code-bg,
    stroke: 1pt + rgb("#dddddd"),
    inset: 8pt,
    breakable: false,
  )

  show quote.where(block: true): it => block(
    stroke: (left: 3pt + rgb("#888888")),
    inset: (left: 10pt, y: 3pt),
    breakable: false,
    text(fill: rgb("#333333"), it.body),
  )

  // #767676 clears the 3:1 non-text contrast ratio (WCAG 1.4.11) against white,
  // so the gridlines that delineate cells stay perceivable.
  set table(stroke: 1pt + rgb("#767676"), inset: (x: 6pt, y: 4pt), align: left)

  body
}

// Handbook cover page: the product name and its one-line tagline.
#let cover(tagline) = {
  v(30mm)
  align(center)[
    #heading(level: 1, outlined: false, text(size: 30pt)[sigcull])
    #text(size: 13pt, fill: rgb("#444444"), tagline)
  ]
}

// One doc: starts on a fresh page, carries its own language (the Spanish
// handbook includes the English-only ADRs), and opens with its title as H1.
#let article(title: none, lang: none, body) = {
  pagebreak(weak: true)
  set text(lang: lang) if lang != none
  heading(level: 1, title)
  body
}

// Starlight aside (:::note, :::tip, :::caution, :::danger): a left-ruled block
// whose label is real bold text, so the meaning never rides on the color alone.
#let aside(kind: "note", label: none, body) = {
  let color = (
    note: accent,
    tip: rgb("#2d6a2d"),
    caution: rgb("#8a5a00"),
    danger: rgb("#a3221d"),
  ).at(kind)
  block(
    width: 100%,
    inset: (left: 10pt, right: 6pt, y: 6pt),
    stroke: (left: 3pt + color),
    breakable: false,
  )[#strong(label)#parbreak()#body]
}
