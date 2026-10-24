// Bilingual JSON Resume template with a density knob.
//
// Compile with:
//   typst compile --input json_path=/dist/resume-en.json \
//     --input font_size=10 --input v_spacing=0.65 templates/resume.typ out.pdf
//
// font_size and v_spacing are the auto-fit solver's inputs; see
// internal/pdf/autofit.go. Optional: avatar=<root-relative image path>.

#import "style.typ": *

#let src = sys.inputs.at("json_path", default: "resume.json")
#let avatar = sys.inputs.at("avatar", default: "")
#let font_size = float(sys.inputs.at("font_size", default: "10"))
#let v_spacing = float(sys.inputs.at("v_spacing", default: "0.65"))

#let cv = json(src)
#let lang = cv.meta.at("language", default: "en")
#let s = strings(lang)
#let basics = cv.basics

#show: doc => {
  setup(font-size: font_size, v-spacing: v_spacing, name: basics.at("name", default: ""), lang: lang)
  doc
}

#header(basics, lang, avatar: avatar)

// ── Summary ─────────────────────────────────────────────────────────────────
#if basics.at("summary", default: "") != "" [
  #section(s.summary, basics.summary)
]

// ── Experience ──────────────────────────────────────────────────────────────
#let jobs = cv.at("work", default: ())
#if jobs.len() > 0 [
  #section(s.experience, {
    for w in jobs {
      block(below: 6pt)[
        #entry-grid(
          [#text(weight: "bold", size: 10pt)[#w.at("position", default: "")]],
          [#text(size: 8pt, fill: muted)[#dates(w.at("startDate", default: ""), w.at("endDate", default: ""), lang)]],
        )
        #text(size: 9pt, fill: soft)[
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
  #section(s.projects, {
    for p in projects {
      block(below: 5pt)[
        #entry-grid(
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
  #section(s.education, {
    for e in edu {
      block(below: 4pt)[
        #entry-grid(
          [#text(weight: "bold")[#e.at("institution", default: "")]],
          [#text(size: 8pt, fill: muted)[#dates(e.at("startDate", default: ""), e.at("endDate", default: ""), lang)]],
        )
        #text(size: 9pt, fill: soft)[#e.at("studyType", default: "") #e.at("area", default: "")]
        #let courses = e.at("courses", default: ())
        #if courses.len() > 0 [
          #text(size: 8.5pt, fill: muted)[#s.courses: #courses.join(", ")]
        ]
      ]
    }
  })
]

// ── Skills ──────────────────────────────────────────────────────────────────
#let skills = cv.at("skills", default: ())
#if skills.len() > 0 [
  #section(s.skills, {
    for skill in skills [
      #text(weight: "bold")[#skill.at("name", default: "")]
      #if skill.at("keywords", default: ()).len() > 0 [ — #skill.keywords.join(", ")]
      \
    ]
  })
]

// ── Languages ───────────────────────────────────────────────────────────────
#let langs = cv.at("languages", default: ())
#if langs.len() > 0 [
  #section(s.languages, {
    for l in langs [
      - #l.at("language", default: "") — #l.at("fluency", default: "")
    ]
  })
]
