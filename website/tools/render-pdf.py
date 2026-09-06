#!/usr/bin/env python
"""Render a print HTML page to a PDF that is BOTH PDF/A-3a and PDF/UA-1.

WeasyPrint's --pdf-variant stamps only one conformance claim, so we render
PDF/A-3a (archival, and its "a" level carries the accessible tag tree) and use
WeasyPrint's own `finisher` hook to add the PDF/UA-1 identifier into the primary
XMP packet, plus the PDF/A extension-schema description that PDF/A clause
6.6.2.3.1 requires for that identifier. The finisher also merges away the Span
structure elements WeasyPrint wraps around every line of text (see
merge_text_spans). The result validates green on both veraPDF flavours (3a and
ua1). No dependency beyond WeasyPrint's bundled pydyf.

Usage: render-pdf.py <input.html> <output.pdf>
"""

from __future__ import annotations

import os
import re
import sys

import pydyf
import weasyprint

# Expressive Code renders every fenced block as a <figure class="frame"> in which
# each token is its own inline <span>, wrapped in per-line <div>s, alongside a copy
# button and (for terminal frames) a "Terminal window" caption. A browser colors
# and hides all of that through Expressive Code's stylesheet; WeasyPrint cannot
# load that site-absolute stylesheet, so in the PDF the chrome renders raw and,
# worse, every one of those hundreds of spans becomes a Span structure element in
# the tag tree (the PDF/UA + WCAG "parsing" noise PAC reports) and the caption
# tags as a Caption an AI check reads as a stray heading. None of it earns its keep
# in a static, monochrome PDF, so each block is collapsed to a single plain <pre>
# holding just the code text, one clean structure element, before rendering.
_EC_BLOCK = re.compile(r'<div class="expressive-code">.*?</figure></div>', re.S)
_EC_CODE = re.compile(r"<pre[^>]*><code>(.*?)</code></pre>", re.S)
_TAG = re.compile(r"<[^>]+>")


def _flatten_block(match: re.Match[str]) -> str:
    block = match.group(0)
    code = _EC_CODE.search(block)
    if not code:
        return block
    # Each source line is one `<div class="ec-line…">`; split on that whole opening
    # tag (extra classes like a highlight marker allowed) and strip the token
    # markup, leaving the text (HTML entities intact) per line.
    segments = re.split(r'<div class="ec-line[^"]*">', code.group(1))
    lines = [_TAG.sub("", segment) for segment in segments[1:]]
    # Emit a <p>, not a <pre>: WeasyPrint tags <pre> as /NonStruct (literally "no
    # structure", which accessibility checkers flag as content lacking a structural
    # element), while <p> tags as a real /P. print.css restores the preformatted
    # look with white-space: pre-wrap on .ec-flat.
    return '<p class="ec-flat">' + "\n".join(lines) + "</p>"


def flatten_expressive_code(html: str) -> str:
    """Collapse Expressive Code blocks to plain <pre> for a clean PDF tag tree."""
    return _EC_BLOCK.sub(_flatten_block, html)

# Namespaces the injected metadata needs, added to the primary <rdf:RDF>.
NEW_NS = (
    'xmlns:pdfuaid="http://www.aiim.org/pdfua/ns/id/" '
    'xmlns:pdfaExtension="http://www.aiim.org/pdfa/ns/extension/" '
    'xmlns:pdfaSchema="http://www.aiim.org/pdfa/ns/schema#" '
    'xmlns:pdfaProperty="http://www.aiim.org/pdfa/ns/property#" '
)

# The PDF/UA-1 identifier, plus a description of its (non-predefined) schema in
# the PDF/A extension-schema container so PDF/A stays valid.
INJECT = (
    '<rdf:Description rdf:about="" pdfuaid:part="1" />'
    '<rdf:Description rdf:about="">'
    "<pdfaExtension:schemas><rdf:Bag><rdf:li rdf:parseType=\"Resource\">"
    "<pdfaSchema:schema>PDF/UA identification schema</pdfaSchema:schema>"
    "<pdfaSchema:namespaceURI>http://www.aiim.org/pdfua/ns/id/</pdfaSchema:namespaceURI>"
    "<pdfaSchema:prefix>pdfuaid</pdfaSchema:prefix>"
    "<pdfaSchema:property><rdf:Seq><rdf:li rdf:parseType=\"Resource\">"
    "<pdfaProperty:name>part</pdfaProperty:name>"
    "<pdfaProperty:valueType>Integer</pdfaProperty:valueType>"
    "<pdfaProperty:category>internal</pdfaProperty:category>"
    "<pdfaProperty:description>Indicates the conformance level of PDF/UA</pdfaProperty:description>"
    "</rdf:li></rdf:Seq></pdfaSchema:property>"
    "</rdf:li></rdf:Bag></pdfaExtension:schemas>"
    "</rdf:Description>"
)


def add_pdfua_identifier(document, pdf) -> None:
    """Inject the PDF/UA-1 identifier into the primary XMP metadata stream."""
    for obj in pdf.objects:
        chunks = getattr(obj, "stream", None)
        if not chunks:
            continue
        data = b"".join(c if isinstance(c, bytes) else str(c).encode("utf-8") for c in chunks)
        if b"<?xpacket" not in data or b"pdfaid:part" not in data:
            continue
        xmp = data.decode("utf-8")
        if "pdfuaid" in xmp:
            return
        xmp = xmp.replace("<rdf:RDF ", "<rdf:RDF " + NEW_NS, 1)
        xmp = xmp.replace("</rdf:RDF>", INJECT + "</rdf:RDF>", 1)
        obj.stream = [xmp.encode("utf-8")]
        obj.compress = False
        return
    raise SystemExit("render-pdf: XMP metadata stream not found in PDF")


# Indirect references serialize as `b"12 0 R"`; pull out the object number.
_OBJ_REF = re.compile(rb"^(\d+) 0 R$")


def _ref_number(ref):
    match = _OBJ_REF.match(ref) if isinstance(ref, bytes) else None
    return int(match.group(1)) if match else None


def merge_text_spans(document, pdf) -> None:
    """Hoist WeasyPrint's per-line text Spans into their parent structure element.

    WeasyPrint wraps the text of every line box in its own /Span structure element
    (even a one-line paragraph becomes P > Span). That is valid, and veraPDF
    accepts it, but accessibility checkers such as PAC flag each Span as a
    possibly-redundant element. Because these Spans only ever wrap one run of text
    and carry no attributes of their own, the run can attach straight to the parent
    with no loss of meaning, exactly as WeasyPrint already does for link text.

    For each such Span this rewrites the standard tag-tree wiring: swap the Span
    reference for its marked-content id in the parent's /K, repoint the ParentTree
    slot for that id at the parent, and empty the now-orphaned Span object. It
    touches only the PDF structure tree and number tree (both ISO 32000, not
    WeasyPrint internals), so it is stable across WeasyPrint versions. Spans that
    carry extra semantics (a language, alt text, and so on) are left untouched.
    """
    # Reach the structure tree and its ParentTree defensively: if a future
    # WeasyPrint lays these out differently, leave the Spans in place rather than
    # fail the render. The PDF stays valid either way, just with the extra Spans.
    root_number = _ref_number(pdf.catalog.get("StructTreeRoot"))
    if root_number is None:
        return
    tree_number = _ref_number(pdf.objects[root_number].get("ParentTree"))
    if tree_number is None:
        return
    nums = pdf.objects[tree_number].get("Nums")
    if not isinstance(nums, pydyf.Array):
        return
    # Nums is a flat [page_number, per-page Array, ...] list where the per-page
    # Array maps a marked-content id (its index) to the element that owns it.
    page_maps = {
        nums[i]: nums[i + 1]
        for i in range(0, len(nums) - 1, 2)
        if isinstance(nums[i + 1], pydyf.Array)
    }
    page_number_of = {ref: number for number, ref in enumerate(getattr(pdf, "page_references", []))}

    for span in pdf.objects:
        if not (isinstance(span, pydyf.Dictionary) and span.get("S") == "/Span"):
            continue
        if set(span.keys()) - {"Type", "S", "K", "Pg", "P"}:
            continue
        kids = span["K"]
        if not (isinstance(kids, pydyf.Array) and len(kids) == 1 and isinstance(kids[0], int)):
            continue
        mcid = kids[0]
        parent_number = _ref_number(span["P"])
        if parent_number is None:
            continue
        parent = pdf.objects[parent_number]
        parent_kids = parent["K"]
        try:
            index = list(parent_kids).index(span.reference)
        except ValueError:
            continue
        parent_kids[index] = mcid
        page_map = page_maps.get(page_number_of.get(span["Pg"]))
        if page_map is not None and 0 <= mcid < len(page_map) and page_map[mcid] == span.reference:
            page_map[mcid] = parent.reference
        # The Span is now unreferenced; clear it so nothing (not even a raw object
        # scan) still sees a Span structure element.
        span.clear()


def finish_pdf(document, pdf) -> None:
    """Finisher: strip redundant text Spans, then stamp the PDF/UA-1 identifier."""
    merge_text_spans(document, pdf)
    add_pdfua_identifier(document, pdf)


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit("usage: render-pdf.py <input.html> <output.pdf>")
    src, dst = sys.argv[1], sys.argv[2]
    with open(src, encoding="utf-8") as handle:
        html = flatten_expressive_code(handle.read())
    # base_url is the source file so any relative asset resolves exactly as it
    # would when reading the file directly.
    weasyprint.HTML(string=html, base_url=os.path.abspath(src)).write_pdf(
        dst, pdf_variant="pdf/a-3a", finisher=finish_pdf
    )


if __name__ == "__main__":
    main()
