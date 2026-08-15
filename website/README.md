# sigcull website

The sigcull marketing and documentation site, built with Hugo (extended).

The extended build is pinned in the repository `mise.toml` as
`hugo-extended`. Because a global mise config may pin a different `hugo`, run
Hugo through mise so the extended 0.165 binary is used.

## Develop

```sh
mise x -- hugo server --source website
```

## Build

```sh
mise x -- hugo --source website --minify
```

Output is written to `website/public`.

## Layout

```
hugo.yaml            site config, languages (en, es), taxonomies
assets/css           stylesheet, processed by Hugo Pipes
layouts              templates (baseof, home, page, section, partials)
content/en           English content
content/es           Spanish content
i18n                 interface strings per language
static               favicon and other verbatim files
```
