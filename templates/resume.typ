// Bilingual JSON Resume template.
//
// Compile with:
//   typst compile --input json_path=dist/resume-en.json templates/resume.typ out.pdf
//
// Optional inputs: avatar=<local image path>. Section headings follow
// meta.language, so the same template renders an English and a German CV.

#let src = sys.inputs.at("json_path", default: "resume.json")
#let avatar = sys.inputs.at("avatar", default: "")
#let cv = json(src)
#let lang = cv.meta.at("language", default: "en")
#let de = lang == "de"

#let h = (
  summary: if de { "Profil" } else { "Summary" },
  experience: if de { "Berufserfahrung" } else { "Experience" },
  projects: if de { "Projekte" } else { "Projects" },
  education: if de { "Ausbildung" } else { "Education" },
  skills: if de { "Kompetenzen" } else { "Skills" },
  languages: if de { "Sprachen" } else { "Languages" },
  courses: if de { "Kurse" } else { "Courses" },
)

#let accent = rgb("#1f4e79")
#let muted = rgb("#5a5a5a")

#set page(
  paper: "a4",
  margin: (x: 15mm, y: 12mm),
  footer: context [
    #set text(size: 7pt, fill: rgb("#8a8a8a"))
    #align(center)[
      #cv.basics.at("name", default: "") · #lang · page #counter(page).display()
    ]
  ],
)
#set text(size: 9.5pt, font: ("DejaVu Sans", "Helvetica", "Arial"), fill: rgb("#1a1a1a"))
#set par(justify: true, leading: 0.55em)
#set heading(numbering: none)
#set list(indent: 0.9em, marker: [•])

#let basics = cv.basics
#let alt = basics.at("altLabels", default: ())
#let loc = basics.at("location", default: (:))

#let section(title, body) = {
  v(7pt)
  block(below: 3pt)[
    #text(size: 10.5pt, weight: "bold", fill: accent)[#title]
    #v(-3pt)
    #line(length: 100%, stroke: 0.6pt + accent)
  ]
  body
}

#let dates(start, end) = {
  let a = start
  let b = if end == "" { if de { "heute" } else { "present" } } else { end }
  if a == "" { b } else { a + " – " + b }
}

// ── Header ──────────────────────────────────────────────────────────────────
#grid(
  columns: if avatar == "" { (1fr,) } else { (1fr, auto) },
  align: (left, right),
  [
    #align(center)[
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
      ]
    ]
  ],
  if avatar != "" {
    image(avatar, width: 26mm)
  },
)

// ── Summary ─────────────────────────────────────────────────────────────────
#if basics.at("summary", default: "") != "" [
  #section(h.summary, basics.at("summary"))
]

// ── Experience ──────────────────────────────────────────────────────────────
#let jobs = cv.at("work", default: ())
#if jobs.len() > 0 [
  #section(h.experience, {
    for w in jobs {
      block(below: 6pt)[
        #grid(
          columns: (1fr, auto),
          [#text(weight: "bold", size: 10pt)[#w.at("position", default: "")]],
          [#text(size: 8pt, fill: muted)[#dates(w.at("startDate", default: ""), w.at("endDate", default: ""))]],
        )
        #text(size: 9pt, fill: rgb("#333333"))[
          #w.at("name", default: "")
          #if w.at("url", default: "") != "" [ · #w.url]
        ]
        #if w.at("summary", default: "") != "" [
          #v(1pt)
          #w.summary
        ]
        #if w.at("description", default: "") != "" [
          #v(1pt)
          #w.description
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

// ── Projects ────────────────────────────────────────────────────────────────
#let projects = cv.at("projects", default: ())
#if projects.len() > 0 [
  #section(h.projects, {
    for p in projects {
      block(below: 5pt)[
        #grid(
          columns: (1fr, auto),
          [#text(weight: "bold", size: 10pt)[#p.at("name", default: "")]],
          [#text(size: 8pt, fill: muted)[#p.at("roles", default: ()).join(" · ")]],
        )
        #if p.at("url", default: "") != "" [
          #text(size: 8pt, fill: muted)[#p.url]
        ]
        #if p.at("description", default: "") != "" [
          #v(1pt)
          #p.description
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

// ── Education ───────────────────────────────────────────────────────────────
#let edu = cv.at("education", default: ())
#if edu.len() > 0 [
  #section(h.education, {
    for e in edu {
      block(below: 4pt)[
        #grid(
          columns: (1fr, auto),
          [#text(weight: "bold")[#e.at("institution", default: "")]],
          [#text(size: 8pt, fill: muted)[#dates(e.at("startDate", default: ""), e.at("endDate", default: ""))]],
        )
        #text(size: 9pt, fill: rgb("#333333"))[
          #e.at("studyType", default: "") #e.at("area", default: "")
        ]
        #let courses = e.at("courses", default: ())
        #if courses.len() > 0 [
          #text(size: 8.5pt, fill: muted)[#h.courses: #courses.join(", ")]
        ]
      ]
    }
  })
]

// ── Skills ──────────────────────────────────────────────────────────────────
#let skills = cv.at("skills", default: ())
#if skills.len() > 0 [
  #section(h.skills, {
    for s in skills [
      #text(weight: "bold")[#s.at("name", default: "")]
      #if s.at("keywords", default: ()).len() > 0 [ — #s.keywords.join(", ")]
      \
    ]
  })
]

// ── Languages ───────────────────────────────────────────────────────────────
#let langs = cv.at("languages", default: ())
#if langs.len() > 0 [
  #section(h.languages, {
    for l in langs [
      - #l.at("language", default: "") — #l.at("fluency", default: "")
    ]
  })
]
