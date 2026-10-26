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
	Examples    = "Complete examples"
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
	{
		ID: "ex-three-tier", Title: "Three-tier web application", Category: Examples,
		Description: "Users, a CDN, a load balancer, redundant web and app servers, and a primary database with a read replica.",
		Code: `direction: right

users: Users { shape: person }
cdn: CDN { shape: cloud }
lb: Load balancer

web: Web tier {
  web1: Web server 1
  web2: Web server 2
}
app: App tier {
  app1: App server 1
  app2: App server 2
}
data: Data tier {
  primary: Primary database { shape: cylinder }
  replica: Read replica { shape: cylinder }
  primary -> replica: replicates
}

users -> cdn: HTTPS
cdn -> lb: cache miss
lb -> web.web1
lb -> web.web2
web.web1 -> app.app1
web.web2 -> app.app2
app.app1 -> data.primary: writes
app.app2 -> data.replica: reads
`,
	},
	{
		ID: "ex-kubernetes", Title: "Kubernetes service", Category: Examples,
		Description: "An ingress controller routing to a service backed by three pods that read configuration and secrets.",
		Code: `internet: Internet { shape: cloud }

cluster: Kubernetes cluster {
  ingress: Ingress controller
  service: shop-api Service
  deploy: shop-api Deployment {
    pod1: Pod 1
    pod2: Pod 2
    pod3: Pod 3
  }
  config: ConfigMap { shape: page }
  secret: Secret { shape: page }

  ingress -> service
  service -> deploy.pod1
  service -> deploy.pod2
  service -> deploy.pod3
  deploy.pod1 -> config: reads
  deploy.pod1 -> secret: reads
}

internet -> cluster.ingress: HTTPS
`,
	},
	{
		ID: "ex-kubernetes-platform", Title: "Kubernetes platform with an admission webhook", Category: Examples,
		Description: "The API server asks a validating admission webhook to approve workloads before they are stored. Around it: a second service the main one hands work to, Helm-installed platform components, and services outside the cluster.",
		Code: `direction: down

engineer: Platform engineer { shape: person }
internet: Internet { shape: cloud }

cluster: Kubernetes cluster {
  control: control plane {
    apiserver: kube-apiserver
    etcd: etcd { shape: cylinder }
  }

  policy: policy namespace {
    hook-svc: policy-webhook Service
    hook: policy-webhook Deployment {
      pod1: Pod 1
      pod2: Pod 2
    }
  }

  edge: ingress-nginx (Helm chart) {
    controller: Ingress controller
  }

  shop: shop namespace {
    api-svc: shop-api Service
    api: shop-api Deployment {
      pod1: Pod 1
      pod2: Pod 2
    }
    worker-svc: shop-worker Service
    worker: shop-worker Deployment {
      pod1: Pod 1
      pod2: Pod 2
    }
    config: ConfigMap { shape: page }
    secret: Secret { shape: page }
  }

  platform: platform (Helm charts) {
    certs: cert-manager
    metrics: Prometheus
    eso: External Secrets Operator
  }

  control.apiserver -> control.etcd: stores objects
  control.apiserver -> policy.hook-svc: AdmissionReview {
    style.stroke-dash: 4
  }
  policy.hook-svc -> policy.hook
  platform.certs -> policy.hook: issues the serving certificate
  platform.certs -> edge.controller: issues TLS certificates

  edge.controller -> shop.api-svc: routes /api
  shop.api-svc -> shop.api
  shop.api -> shop.worker-svc: hands off jobs
  shop.worker-svc -> shop.worker
  shop.api -> shop.config: reads
  shop.api -> shop.secret: reads

  platform.metrics -> shop.api: scrapes
  platform.metrics -> shop.worker: scrapes
  platform.eso -> shop.secret: keeps in sync
}

database: Managed Postgres { shape: cylinder }
vault: Secrets manager { shape: cloud }

engineer -> cluster.control.apiserver: kubectl apply
internet -> cluster.edge.controller: HTTPS
cluster.shop.api -> database: SQL
cluster.shop.worker -> database: SQL
cluster.platform.eso -> vault: fetches secrets
`,
	},
	{
		ID: "ex-ci-cd", Title: "CI/CD pipeline", Category: Examples,
		Description: "From a push, through build, test and scan stages, to an image registry and on to staging and production.",
		Code: `direction: right

dev: Developer { shape: person }
repo: Git repository

ci: CI pipeline {
  build: Build
  test: Test
  scan: Security scan
  build -> test -> scan
}

registry: Image registry { shape: cylinder }
staging: Staging
prod: Production

dev -> repo: push
repo -> ci.build: trigger
ci.scan -> registry: publish image
registry -> staging: deploy
staging -> prod: promote after approval
`,
	},
	{
		ID: "ex-event-driven", Title: "Event-driven services", Category: Examples,
		Description: "One service publishes an event to a broker; three consumers react, and failures land in a dead-letter queue.",
		Code: `direction: right

orders: Order service
bus: Message broker { shape: queue }
billing: Billing service
inventory: Inventory service
notify: Notification service
dlq: Dead-letter queue { shape: queue }

orders -> bus: OrderPlaced
bus -> billing
bus -> inventory
bus -> notify
billing -> dlq: failed after retries {
  style: {
    stroke-dash: 4
    stroke: "#c0392b"
  }
}
`,
	},
	{
		ID: "ex-microservices", Title: "Microservices behind a gateway", Category: Examples,
		Description: "An API gateway in front of four services, each with its own datastore, sharing a cache.",
		Code: `direction: right

client: Client app { shape: person }
gateway: API gateway

services: Services {
  auth: Auth
  users: Users
  catalog: Catalog
  orders: Orders
}

stores: Datastores {
  auth-db: Auth DB { shape: cylinder }
  users-db: Users DB { shape: cylinder }
  catalog-db: Catalog DB { shape: cylinder }
  orders-db: Orders DB { shape: cylinder }
  cache: Shared cache { shape: cylinder }
}

client -> gateway: HTTPS
gateway -> services.auth
gateway -> services.users
gateway -> services.catalog
gateway -> services.orders
services.auth -> stores.auth-db
services.users -> stores.users-db
services.catalog -> stores.catalog-db
services.orders -> stores.orders-db
services.catalog -> stores.cache: reads
`,
	},
	{
		ID: "ex-oauth", Title: "Sign-in with an identity provider", Category: Examples,
		Description: "A sequence diagram of the OAuth authorization-code flow between a user, a web app, an identity provider and an API.",
		Code: `shape: sequence_diagram

user: User
app: Web app
idp: Identity provider
api: API

user -> app: click sign in
app -> idp: redirect with client id
idp -> user: show login page
user -> idp: enter credentials
idp -> app: authorization code
app -> idp: exchange code for token
idp -> app: access token
app -> api: request with token
api -> app: protected data
app -> user: show the page
`,
	},
	{
		ID: "ex-office-network", Title: "Office network", Category: Examples,
		Description: "A router, firewall and switch connecting servers, Wi-Fi and laptops to the internet.",
		Code: `internet: Internet { shape: cloud }

office: Office network {
  router: Router
  firewall: Firewall
  switch: Switch
  wifi: Wi-Fi access point
  servers: Servers {
    files: File server
    printing: Print server
  }
  laptops: Laptops

  router -> firewall -> switch
  switch -> wifi
  switch -> servers.files
  switch -> servers.printing
  wifi -> laptops
}

internet <-> office.router
`,
	},
	{
		ID: "ex-database-cluster", Title: "Highly available database", Category: Examples,
		Description: "A connection pooler sending writes to a primary and reads to two replicas, with nightly backups to object storage.",
		Code: `direction: down

app: Application
pooler: Connection pooler

cluster: Database cluster {
  primary: Primary { shape: cylinder }
  replica1: Replica 1 { shape: cylinder }
  replica2: Replica 2 { shape: cylinder }
  primary -> replica1: streams changes
  primary -> replica2: streams changes
}

backup: Object storage { shape: cloud }

app -> pooler
pooler -> cluster.primary: writes
pooler -> cluster.replica1: reads
pooler -> cluster.replica2: reads
cluster.primary -> backup: nightly backup
`,
	}, {
		ID: "ex-flowchart", Title: "Flowchart: why is the build red?", Category: Examples,
		Description: "Decision diamonds and rounded start and end points, following a team's very relatable troubleshooting routine.",
		Code: `direction: down

start: The build is red { shape: oval }
local: Does it fail on your machine too? { shape: diamond }
flaky: Does it pass if you re-run it? { shape: diamond }
fix: Fix the test
quarantine: Quarantine the flaky test and file a ticket
env: Compare the CI and local environments
revert: Revert the last merge
done: The build is green { shape: oval }

start -> local
local -> fix: yes
local -> flaky: no
flaky -> quarantine: yes
flaky -> env: no
env -> revert: no difference found
env -> done: found it
fix -> done
quarantine -> done
revert -> done
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
