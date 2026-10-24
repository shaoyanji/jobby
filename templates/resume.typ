// Bilingual JSON Resume template.
// Compile with: typst compile --input json_path=dist/active-en.json
#let src = sys.inputs.at("json_path", default: "resume.json")
#let cv = json(src)
#let lang = cv.meta.at("language", default: "en")
#let de = lang == "de"

#let h = (
  experience: if de { "Berufserfahrung" } else { "Experience" },
  projects: if de { "Projekte" } else { "Projects" },
  education: if de { "Ausbildung" } else { "Education" },
  skills: if de { "Kompetenzen" } else { "Skills" },
  languages: if de { "Sprachen" } else { "Languages" },
  summary: if de { "Profil" } else { "Summary" },
)

#set page(
  paper: "a4",
  margin: (x: 14mm, y: 12mm),
  footer: context [
    #set text(size: 7pt, fill: rgb("#8a8a8a"))
    #align(center)[
      #cv.basics.at("name", default: "") · resumetui · #lang \
    ]
  ],
)
#set text(size: 9.5pt, font: ("DejaVu Sans", "Helvetica", "Arial"), fill: rgb("#1a1a1a"))
#set par(justify: true, leading: 0.55em)
#set heading(numbering: none)

#let basics = cv.basics
#let alt = basics.at("altLabels", default: ())

#let section(title, body) = {
  v(7pt)
  block(below: 2pt)[
    #text(size: 11pt, weight: "bold", fill: rgb("#1f4e79"))[#title]
    #v(-2pt)
    #line(length: 100%, stroke: 0.5pt + rgb("#1f4e79"))
  ]
  body
}

#align(center)[
  #text(size: 20pt, weight: "bold")[#basics.at("name", default: "")]
  #v(2pt)
  #text(size: 11pt, fill: rgb("#1f4e79"))[#basics.at("label", default: "")]
  #if alt.len() > 0 [
    #v(1pt)
    #text(size: 8.5pt, fill: rgb("#555555"))[#alt.join(" · ")]
  ]
  #v(3pt)
  #text(size: 8.5pt, fill: rgb("#555555"))[
    #basics.at("email", default: "")
    #if basics.at("phone", default: "") != "" [ · #basics.at("phone")]
    #if basics.at("url", default: "") != "" [ · #basics.at("url")]
    #if basics.at("location", default: (:)).at("city", default: "") != "" [ · #basics.location.city]
  ]
]

#if basics.at("summary", default: "") != "" [
  #section(h.summary, basics.at("summary"))
]

#let jobs = cv.at("work", default: ())
#if jobs.len() > 0 [
  #section(h.experience, {
    for w in jobs {
      let start = w.at("startDate", default: "")
      let end = w.at("endDate", default: "")
      let when = if end == "" { start + " – " } else { start + " – " + end }
      block(below: 5pt)[
        #grid(
          columns: (1fr, auto),
          [#text(weight: "bold", size: 10pt)[#w.at("position", default: "")]],
          [#text(size: 8pt, fill: rgb("#555555"))[#when]],
        )
        #text(size: 9pt, fill: rgb("#333333"))[#w.at("name", default: "")]
        #if w.at("summary", default: "") != "" [
          #v(1pt)
          #w.at("summary")
        ]
        #let hs = w.at("highlights", default: ())
        #if hs.len() > 0 [
          #v(1pt)
          #for x in hs [
            - #x
          ]
        ]
      ]
    }
  })
]

#let projects = cv.at("projects", default: ())
#if projects.len() > 0 [
  #section(h.projects, {
    for p in projects {
      block(below: 5pt)[
        #text(weight: "bold", size: 10pt)[#p.at("name", default: "")]
        #if p.at("url", default: "") != "" [
          #text(size: 8pt, fill: rgb("#555555"))[ · #p.at("url")]
        ]
        #if p.at("description", default: "") != "" [
          #v(1pt)
          #p.at("description")
        ]
        #let hs = p.at("highlights", default: ())
        #if hs.len() > 0 [
          #v(1pt)
          #for x in hs [
            - #x
          ]
        ]
      ]
    }
  })
]

#let edu = cv.at("education", default: ())
#if edu.len() > 0 [
  #section(h.education, {
    for e in edu {
      block(below: 3pt)[
        #grid(
          columns: (1fr, auto),
          [#text(weight: "bold")[#e.at("institution", default: "")]],
          [#text(size: 8pt, fill: rgb("#555555"))[#e.at("startDate", default: "") #if e.at("endDate", default: "") != "" [ – #e.endDate]]],
        )
        #text(size: 9pt, fill: rgb("#333333"))[#e.at("studyType", default: "") #e.at("area", default: "")]
      ]
    }
  })
]

#let skills = cv.at("skills", default: ())
#if skills.len() > 0 [
  #section(h.skills, {
    for s in skills [
      #text(weight: "bold")[#s.at("name", default: "")] #if s.at("keywords", default: ()).len() > 0 [ — #s.keywords.join(", ")] \
    ]
  })
]

#let langs = cv.at("languages", default: ())
#if langs.len() > 0 [
  #section(h.languages, {
    for l in langs [
      - #l.at("language", default: "") — #l.at("fluency", default: "") \
    ]
  })
]
