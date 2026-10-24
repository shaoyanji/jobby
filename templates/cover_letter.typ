// Cover letter (Anschreiben) on the same header, palette and density knobs as
// templates/resume.typ. The signature is stamped afterwards by pdfcpu, exactly
// like the CV, so both documents carry the same mark in the same corner.
//
// Compile with:
//   typst compile --input json_path=/…/letter.json --input font_size=10 \
//     --input v_spacing=0.65 templates/cover_letter.typ Cover_Letter.pdf

#import "style.typ": *

#let src = sys.inputs.at("json_path", default: "letter.json")
#let avatar = sys.inputs.at("avatar", default: "")
#let font_size = float(sys.inputs.at("font_size", default: "10"))
#let v_spacing = float(sys.inputs.at("v_spacing", default: "0.65"))

#let payload = json(src)
#let basics = payload.basics
#let letter = payload.letter
#let lang = payload.meta.at("language", default: "en")

#show: doc => {
  setup(font-size: font_size, v-spacing: v_spacing, name: basics.at("name", default: ""), lang: lang)
  doc
}

#header(basics, lang, avatar: avatar)

#v(14pt)

#grid(
  columns: (1fr, auto),
  align: (left, right),
  [
    #text(weight: "bold")[#letter.at("company", default: "")]
    #if letter.at("role", default: "") != "" [
      #linebreak()
      #text(size: 9pt, fill: muted)[#letter.role]
    ]
  ],
  [#text(size: 9pt, fill: muted)[#payload.at("date", default: "")]],
)

#v(10pt)

#if letter.at("salutation", default: "") != "" [
  #letter.salutation
  #parbreak()
]

#if letter.at("opening", default: "") != "" [
  #letter.opening
  #parbreak()
]

#for p in letter.at("body_paragraphs", default: ()) [
  #p
  #parbreak()
]

#if letter.at("closing", default: "") != "" [
  #letter.closing
]

#v(30pt)
#text(weight: "bold")[#basics.at("name", default: "")]
