// Shared style for the CV and the cover letter.
//
// One palette, one header, one date format, so a package looks like one
// document rather than two templates that drifted apart.
//
// Both templates call:
//   #show: doc => { setup(...); doc }
// which scopes the page/text/paragraph rules to the whole document.

#let accent = rgb("#1f4e79")
#let muted = rgb("#5a5a5a")
#let soft = rgb("#333333")
#let rule = rgb("#8a8a8a")
#let fonts = ("DejaVu Sans", "Helvetica", "Arial")

#let labels = (
  en: (
    summary: "Summary",
    experience: "Experience",
    projects: "Projects",
    education: "Education",
    skills: "Skills",
    languages: "Languages",
    courses: "Courses",
  ),
  de: (
    summary: "Profil",
    experience: "Berufserfahrung",
    projects: "Projekte",
    education: "Ausbildung",
    skills: "Kompetenzen",
    languages: "Sprachen",
    courses: "Kurse",
  ),
)

#let strings(lang) = if lang == "de" { labels.de } else { labels.en }

#let setup(font-size: 10, v-spacing: 0.65, name: "", lang: "en") = {
  set page(
    paper: "a4",
    margin: (x: 15mm, y: 12mm),
    footer: context [
      #set text(size: 7pt, fill: rule)
      #align(center)[#name · #lang · #counter(page).display()]
    ],
  )
  set text(size: font-size * 1pt, font: fonts, fill: rgb("#1a1a1a"))
  set par(justify: true, leading: v-spacing * 1em)
  set heading(numbering: none)
  set list(indent: 0.9em, marker: [•])
}

// header is the identity block: name, persona, alternatives, contact. The
// cover letter reuses it verbatim, which is what makes the package look
// authored rather than assembled.
#let header(basics, lang, avatar: "") = {
  let alt = basics.at("altLabels", default: ())
  let loc = basics.at("location", default: (:))
  grid(
    columns: if avatar == "" { (1fr,) } else { (1fr, auto) },
    align: (center + horizon, right + horizon),
    [
      #text(size: 20pt, weight: "bold")[#basics.at("name", default: "")]
      #v(2pt)
      #text(size: 11pt, fill: accent)[#basics.at("label", default: "")]
      #if alt.len() > 0 [
        #v(1pt)
        #text(size: 8.5pt, fill: muted)[#alt.join(" · ")]
      ]
      #v(3pt)
      #text(size: 8.5pt, fill: muted)[
        #basics.at("email", default: "")
        #if basics.at("phone", default: "") != "" [ · #basics.phone]
        #if basics.at("url", default: "") != "" [ · #basics.url]
        #if loc.at("city", default: "") != "" [ · #loc.city]
        #if loc.at("countryCode", default: "") != "" [ · #loc.countryCode]
      ]
    ],
    if avatar != "" { image(avatar, width: 26mm) },
  )
}

#let section(title, body) = {
  v(7pt)
  block(below: 3pt)[
    #text(size: 10.5pt, weight: "bold", fill: accent)[#title]
    #v(-3pt)
    #line(length: 100%, stroke: 0.6pt + accent)
  ]
  body
}

// dates is the right-hand column of every entry grid. It never wraps, which is
// what keeps long German job titles from pushing the dates off the margin.
#let dates(start, end, lang) = {
  let today = if lang == "de" { "heute" } else { "present" }
  let b = if end == "" { today } else { end }
  if start == "" { b } else { start + " – " + b }
}

// entry-grid locks a left block against a right-aligned meta column. The
// parameters are not called left/right: those names are the alignment values
// and would shadow them inside the function.
#let entry-grid(main, meta) = {
  grid(columns: (1fr, auto), align: (start + top, end + top), main, meta)
}
