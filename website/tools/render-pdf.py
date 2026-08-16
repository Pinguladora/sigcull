#!/usr/bin/env python
"""Render a print HTML page to a PDF that is BOTH PDF/A-3a and PDF/UA-1.

WeasyPrint's --pdf-variant stamps only one conformance claim, so we render
PDF/A-3a (archival, and its "a" level carries the accessible tag tree) and use
WeasyPrint's own `finisher` hook to add the PDF/UA-1 identifier into the primary
XMP packet, plus the PDF/A extension-schema description that PDF/A clause
6.6.2.3.1 requires for that identifier. The result validates green on both
veraPDF flavours (3a and ua1). No dependency beyond WeasyPrint's bundled pydyf.

Usage: render-pdf.py <input.html> <output.pdf>
"""

from __future__ import annotations

import sys

import weasyprint

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


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit("usage: render-pdf.py <input.html> <output.pdf>")
    src, dst = sys.argv[1], sys.argv[2]
    weasyprint.HTML(src).write_pdf(dst, pdf_variant="pdf/a-3a", finisher=add_pdfua_identifier)


if __name__ == "__main__":
    main()
