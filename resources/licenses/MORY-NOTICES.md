# Third-party notices

Mory is licensed under MIT. Its native interface uses MyGo (MIT), and JavaScript
transforms and equation typesetting run in moejs (Apache-2.0). The complete
Unicode, ICU and V8 notices distributed with moejs are included alongside it.

The native equation layout, built-in document styles and structured-document
fixture include MIT-licensed adaptations by s1dashu, copyright 2026. The author's
copyright and permission notice are preserved in `resources/licenses/s1dashu-MIT`.

KaTeX 0.16.22 and its bundled fonts provide mathematical typesetting. Their MIT
notice is in `resources/licenses/katex-LICENSE`. CSS syntax is parsed by
`github.com/tdewolff/parse/v2`; its MIT notice is included as `css-parser-MIT`.

Luo Regular 0.4.12 supplies Chinese glyphs in the native interface and built-in
document themes. The unmodified font is bundled at
`internal/typography/assets/Luo-Regular.ttf`, from
[`tw93/Luo` commit `c941c0b522ab32a5f8c5f817540c12709f898b40`](https://github.com/tw93/Luo/tree/c941c0b522ab32a5f8c5f817540c12709f898b40)
(retrieved November 14, 2026). Its SHA-256 is
`27449a8ded7f1c51e36f2cc2db3569910cd85b97ff16ce8e026ce3de2a14f140`.
Copyright 2026 the Luo project authors, with portions copyright Lxgw. The
SIL Open Font License 1.1 and upstream attribution are preserved verbatim in
`resources/licenses/Luo-OFL.txt`. Luo remains a preview font; unavailable
characters use macOS fallback fonts. Registration is local to the Mory process.

MyGo, moejs, purego, go-text/typesetting, and the Go x/image, x/net and x/text
module licenses are included in `resources/licenses`. Exact module versions and
checksums are locked in `go.mod` and `go.sum`. Packaging copies these notices into
`Mory.app/Contents/Resources/licenses`.

The Mory portrait icon was generated specifically for this application. No
commercial icon pack or third-party application branding is distributed.
