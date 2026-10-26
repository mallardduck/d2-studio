// Package snippets is the built-in gallery of small D2 examples. They are
// written for this app (the syntax is D2's, the wording is ours) and every one
// is compiled by the tests, so the gallery cannot ship an example that does
// not work.
package snippets

import "slices"

// Snippet is one example the user can preview, copy or insert.
type Snippet struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Description string `json:"description"`
	// Code is the D2 source. It always ends with a newline.
	Code string `json:"code"`
	// Remote is true when the example refers to an image on the web, which
	// the thumbnail does not load.
	Remote bool `json:"remote"`
}

// Categories in display order.
const (
	Basics      = "Basics"
	Shapes      = "Shapes & connections"
	Structure   = "Structure"
	Text        = "Text & media"
	Diagrams    = "Special diagrams"
	Styling     = "Styling & layout"
	Interactive = "Interactive"
	Reuse       = "Reuse"
)

var all = []Snippet{
	{
		ID: "first-connection", Title: "A first connection", Category: Basics,
		Description: "Two shapes joined by an arrow. A name you mention becomes a shape.",
		Code:        "client -> server\n",
	},
	{
		ID: "labels", Title: "Labels", Category: Basics,
		Description: "Text after a colon labels a shape or a connection.",
		Code: `api: Orders API
store: Orders database
api -> store: SELECT order
`,
	},
	{
		ID: "key-names", Title: "Ways to name a shape", Category: Basics,
		Description: "Keys can contain spaces and underscores. Quote a key that contains punctuation.",
		Code: `checkout
checkout_service
checkout worker
"checkout (v2)"
`,
	},
	{
		ID: "shape-types", Title: "Shape types", Category: Shapes,
		Description: "Set shape to change how a node looks.",
		Code: `visitor: { shape: person }
orders: { shape: cylinder }
approve: { shape: diamond }
jobs: { shape: queue }
cdn: { shape: cloud }
`,
	},
	{
		ID: "connections", Title: "Connection kinds", Category: Shapes,
		Description: "Arrows can point either way or both, chain together, or have no head at all.",
		Code: `web -> api -> worker: a chain
primary <-> standby: replicates
lamp -- switch: wired together
`,
	},
	{
		ID: "arrowheads", Title: "Arrowheads", Category: Shapes,
		Description: "Give each end of a connection its own head, shape and label.",
		Code: `author -> book: writes {
  source-arrowhead: 1
  target-arrowhead: many {
    shape: diamond
  }
}
`,
	},
	{
		ID: "containers", Title: "Containers", Category: Structure,
		Description: "Braces nest shapes. A dot reaches inside a container from outside.",
		Code: `backend: {
  auth
  billing
}
frontend.web
frontend.mobile
frontend.web -> backend.auth: log in
frontend.mobile -> backend.auth: log in
`,
	},
	{
		ID: "grid", Title: "Grid layout", Category: Diagrams,
		Description: "grid-rows or grid-columns lays the children of a shape out as a grid.",
		Code: `team: {
  grid-columns: 3
  design
  build
  test
  ship
  measure
  learn
}
`,
	},
	{
		ID: "markdown", Title: "Markdown text", Category: Text,
		Description: "A |md block renders headings, lists and emphasis.",
		Code: `notes: |md
  # Release checklist

  - tag the build
  - publish the notes

  Anything _standard_ in **Markdown** works.
|
`,
	},
	{
		ID: "code-block", Title: "Code block", Category: Text,
		Description: "Name a language after the pipe and the text is shown as code.",
		Code: `handler: |go
  func Health(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
  }
|
`,
	},
	{
		ID: "latex", Title: "Math (LaTeX)", Category: Text,
		Description: "A |latex block typesets a formula.",
		Code: `sum: |latex
  \sum_{i=1}^{n} i = \frac{n(n+1)}{2}
|
`,
	},
	{
		ID: "icons", Title: "Icons and images", Category: Text,
		Description: "icon takes an image URL or a file path. Remote images need “Remote images” switched on for the file.",
		Code: `gateway: {
  icon: https://icons.terrastruct.com/dev%2Fgo.svg
}
logo: {
  shape: image
  icon: https://icons.terrastruct.com/dev%2Fgo.svg
}
`,
		Remote: true,
	},
	{
		ID: "sql-table", Title: "SQL tables", Category: Diagrams,
		Description: "shape: sql_table lists columns and their types. A connection can join two columns.",
		Code: `customers: {
  shape: sql_table
  id: int { constraint: primary_key }
  email: varchar
  created_at: timestamp
}

orders: {
  shape: sql_table
  id: int { constraint: primary_key }
  customer_id: int { constraint: foreign_key }
  total: numeric
}

orders.customer_id -> customers.id
`,
	},
	{
		ID: "class", Title: "UML class", Category: Diagrams,
		Description: "shape: class lists fields and methods. A leading + or - sets visibility.",
		Code: `Cart: {
  shape: class

  +items: "[]Item"
  -discount: float
  # Quote a key that starts with # so it is not read as a comment
  "#recalculate()": void

  +add(item Item): error
  +total(): float
}
`,
	},
	{
		ID: "sequence", Title: "Sequence diagram", Category: Diagrams,
		Description: "shape: sequence_diagram orders messages top to bottom between actors.",
		Code: `shape: sequence_diagram
shopper -> shop: add to cart
shop -> stock: reserve
stock -> shop: ok
shop -> shopper: cart updated
`,
	},
	{
		ID: "style", Title: "Styling a shape and a connection", Category: Styling,
		Description: "The style block controls colors, borders, opacity and more.",
		Code: `service: {
  style: {
    fill: "#e8f3ff"
    stroke: "#1d6fb8"
    stroke-width: 3
    border-radius: 8
  }
}

cache: {
  style: {
    opacity: 0.7
    3d: true
  }
}

service -> cache: {
  style: {
    stroke: "#1d6fb8"
    stroke-dash: 4
    animated: true
  }
}
`,
	},
	{
		ID: "root-style", Title: "Style the whole diagram", Category: Styling,
		Description: "A style block at the top level styles the canvas itself.",
		Code: `style: {
  fill: "#fdf6e3"
  stroke: "#586e75"
  stroke-width: 6
  fill-pattern: dots
}

overview: |md
  # Capacity plan
  Q3 targets are in the table below.
|
`,
	},
	{
		ID: "direction", Title: "Layout direction", Category: Styling,
		Description: "direction sets the flow: up, down, left or right.",
		Code: `direction: right
ingest -> clean -> enrich -> publish
`,
	},
	{
		ID: "title", Title: "Diagram title", Category: Styling,
		Description: "A text shape pinned near an edge makes a title.",
		Code: `title: Order pipeline {
  near: top-center
  shape: text
  style: {
    font-size: 28
    bold: true
  }
}
intake -> pick -> pack
`,
	},
	{
		ID: "tooltip", Title: "Tooltips", Category: Interactive,
		Description: "Shown on hover in an exported SVG viewed in a browser.",
		Code: `worker: { tooltip: Retries three times, then gives up }
dead-letter: { tooltip: Messages that failed every retry }
worker -> dead-letter
`,
	},
	{
		ID: "links", Title: "Links", Category: Interactive,
		Description: "A link makes a shape clickable in an exported SVG.",
		Code: `docs: Documentation {
  link: https://example.com/docs
}
status: Status page {
  link: https://example.com/status
}
docs -> status
`,
	},
	{
		ID: "vars", Title: "Variables", Category: Reuse,
		Description: "Define values once in vars and substitute them with ${name}.",
		Code: `vars: {
  service-name: Billing
  colors: {
    brand: "#7a3fb3"
  }
}

main: ${service-name}
main.style.fill: ${colors.brand}
`,
	},
	{
		ID: "classes", Title: "Classes", Category: Reuse,
		Description: "Define a bundle of attributes once and apply it to many shapes.",
		Code: `classes: {
  important: {
    style: {
      stroke: "#c0392b"
      stroke-width: 4
    }
  }
}

database.class: important
queue.class: important
worker
`,
	},
	{
		ID: "globs", Title: "Globs", Category: Reuse,
		Description: "* matches every shape (or every connection) so one line can style them all.",
		Code: `alpha
beta
gamma

*.style.fill: lightyellow
alpha -> *
`,
	},
}

// All returns the snippets in display order. The slice is a copy.
func All() []Snippet { return slices.Clone(all) }

// ByID returns the snippet with the given ID.
func ByID(id string) (Snippet, bool) {
	for _, s := range all {
		if s.ID == id {
			return s, true
		}
	}
	return Snippet{}, false
}
