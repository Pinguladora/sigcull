---
title: Accessibility
description: How the sigcull documentation site is built and tested for accessibility, and how to report a barrier.
---

sigcull is committed to making its documentation accessible to everyone, whatever their abilities or the technology they use. Accessibility is part of how the site is built, not an afterthought, and it is improved continuously.

## Conformance status

The [Web Content Accessibility Guidelines (WCAG)](https://www.w3.org/WAI/standards-guidelines/wcag/) set the requirements for making web content more accessible. This site is built and tested to meet **WCAG 2.2 Level AA**.

No testing catches every possible barrier, so we describe the site as substantially conformant: when an issue is found, we fix it. If you encounter something that does not reach Level AA, please tell us (see [Feedback](#feedback)).

## Measures we take

- Accessibility is considered during design and development, not added at the end.
- Every change is checked against the tests below before it ships.
- Reports from readers are welcomed and acted on.

## How the site is tested

Every page of the built site is checked, in both the light and dark themes, with:

- **axe-core** run against the WCAG 2.0, 2.1, and 2.2 A and AA rule sets on each page, which also covers ARIA usage and landmark structure.
- **Keyboard-only** operation, so every control and every scrollable code block can be reached and used without a mouse, with no keyboard traps.
- **Reduced motion**: with `prefers-reduced-motion` set, animation and smooth scrolling are turned off.
- **NVDA**, a real screen reader driven with guidepup, exercising the reading and announcement flows.
- **Lighthouse** accessibility audits across the whole site.
- **HTML validation** of the generated markup.

## Accessibility features

- A responsive layout that reflows to a single column and stays usable when zoomed to 200%.
- A skip link to jump straight to the main content.
- Semantic HTML with landmark regions (header, navigation, main, complementary, footer) and a correct, ordered heading structure on every page.
- ARIA used where native HTML is not enough, such as marking the current page in the navigation.
- Full keyboard operability, including focusable, scrollable code blocks, with visible focus indicators.
- Text and interface colors that meet the WCAG AA contrast ratios in both the light and dark themes, with the text itself also meeting the stricter AAA ratio (WCAG 1.4.6).
- Respect for `prefers-reduced-motion`.
- The page language set on every page, so assistive technology reads the English and Spanish content correctly.
- Text alternatives for images.
- Descriptive page titles, an "On this page" outline, and site search, so content can be found in more than one way.
- Downloadable PDFs tagged and validated as **PDF/UA-1** (accessible) and **PDF/A-3a** (archival).

## Technical specifications

Accessibility relies on HTML, WAI-ARIA, CSS, and JavaScript, and on your browser and any assistive technology supporting them. The site is statically generated, so its content can be read without JavaScript.

## Known limitations

We are not aware of outstanding Level AA barriers. Automated tools only cover part of WCAG, so if you find a barrier our testing missed, we would like to hear about it.

## Feedback

If you encounter an accessibility barrier on this site, please open an issue at [github.com/Pinguladora/sigcull/issues](https://github.com/Pinguladora/sigcull/issues). Tell us the page, what went wrong, and the browser and assistive technology you were using. We aim to respond promptly.

## This statement

This statement covers the sigcull documentation site. It was last reviewed on 30 August 2026.
