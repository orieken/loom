# Pilot — the reagent map

One feature taken through every factory on the line, in Saturday's demo app, with each gate answered
by the role that owns it: **analyst**, **developer**, **tester**. It is the proof-of-concept for a
work delivery team. Read `00`–`04` first.

**The feature.** Ye Olde Magic Shop gets a second map view: a 3D CesiumJS globe of an invented planet.
Reagent sites (places adventurers gather ingredients) are marked on it, and saved expeditions are
drawn on it when the view loads. An adventurer draws shapes to plan an expedition and saves it. Shapes
arrive one wave at a time: a straight line first, then circles that enclose sites, then the rest.
Along the way Saturday gains `@orieken/saturday-playwright-cesium`, the sibling of
`saturday-playwright-leaflet`.

## Decided

| Question | Decision |
|---|---|
| 2D or 3D | **3D** (`SceneMode.SCENE3D`). The tests must handle camera position and the far side of the globe. |
| Which globe | **An invented planet**: a perfect sphere with one in-repo equirectangular image. Not Earth, no ion. |
| Who builds it | **loom**, from draft orders in `saturday-monorepo/intake/` (`docs/factory-setup.md` there). |
| Saved through the API | **Yes.** `GET/POST /api/expeditions`, plus seed data, so the map loads with shapes already drawn. |
| Where tests live | **Same repo as the app**, for now. Cross-repo is roadmap L3.76. |
| Runner | Playwright Test, for UI (Saturday) and API (Sunday) alike. |
| First shape | **A straight line, drawn by clicking two points.** |
| Network | **None.** Cesium with a local image, no Cesium ion account. Leaflet made local too. |
| How the factories get built | **By hand first.** Every stage of the pilot is run manually, and each good choice goes in [`05-pilot-decisions.md`](05-pilot-decisions.md) as a candidate skill, tool or rule. |

## Target repo

`/Users/oscarrieken/Projects/Rieken/saturday-monorepo`. App, mock API, packages and tests all live
there, so the floor runs in that repo.

## Prior context (read first)

- `packages/saturday-playwright-leaflet/`: the exemplar package, and its gaps (below)
- `apps/ye-olde-magic-shop/src/views/LocationsView.vue`, `tests/map.spec.ts`
- `apps/mock-api/server.js` and `apps/mock-api/data/`: where reagent sites and expeditions are served and seeded
- `containers/ubi-vnc`, `containers/x11-forwarding-ubi9`: real Chrome under Xvfb; order `ui-suite-real-chrome` picks one
- `packages/saturday-playwright-heatmap/`, `packages/saturday-ml-analyzer/`
- `docs/TODO.md`: "Maps" and "Golden tests"
- commit `b5d3b1e`: how the Leaflet map and package were added

## How the pilot runs

The plans `deliver-spec`, `deliver-tests-saturday` and `deliver-tests-sunday` do not exist yet. Each
stage in `02`–`04` is run by hand, with the existing agents and skills (`/event-storm`, `/five-whys`,
`analyst`, `qa-engineer`) and a real person at each gate. Each stage's output is written in the
typed-state shape its prompt describes, under `docs/specs/<order>/`. Code-producing orders run on the
floor with `deliver-feature` as they become ready.

After every stage, add to `05-pilot-decisions.md`: what was decided, why, and whether it should become
a skill, a tool, a rule or nothing. A factory plan is built once a stage has run by hand at least
twice and its decisions have settled.

## Slice 0 — before any order

1. **Upgrade the framework.** `saturday-monorepo` has ai-assistant-dot-files v3.1.0; `loom factory` and
   intake need the current version. Install it and add `.claude/factory/target.yaml` (shape in `01`).
2. **Repair the golden tests.** Agents left the demo app breaking its own rules. Fix it before any
   factory copies it (`03`, Phase 0; `docs/TODO.md`, "Golden tests").
3. **Run the UI suite in real Chrome.** It runs today in Playwright's bundled Chromium, with no
   channel set. Order `ui-suite-real-chrome` moves it into one of the real-Chrome images under Xvfb
   and asserts WebGL is available. `target.yaml` names that image.
4. **No network.** Cesium with a local image and no ion; the Leaflet view moved to local tiles as its
   own order (`docs/TODO.md`, "Maps").

## The waves

Each wave is one spec order and the orders that build and test it. A wave starts once the previous
wave has shipped. A later wave never edits an earlier spec package: it adds to it, pinned by digest.

### Wave 1: the globe, seeded shapes, and a line

| | Order | Plan | Gates → role | Depends on |
|---|---|---|---|---|
| A1 | `reagent-map-spec` | `deliver-spec` (by hand) | problem → requester; domain, questions → **analyst**; scenarios → **tester** | slice 0 |
| C1 | `saturday-playwright-cesium` | `deliver-feature` | design, ship → **tester** | slice 0 |
| D1 | `reagent-map-view` | `deliver-feature` | design, security, ship → **developer** | A1, C1 seam contract |
| F1 | `reagent-map-api-tests` | `deliver-tests-sunday` (by hand) | testability → **developer**; model, ship → **tester** | A1, D1 |
| E1 | `reagent-map-ui-tests` | `deliver-tests-saturday` (by hand) | testability → **developer**; model, ship → **tester** | A1, C1, D1 |

Wave 1 covers: the view loads the land and the reagent sites; seeded expeditions render as lines in
the right places; the camera can fly to a site; a user draws a line by clicking two points, saves it,
and sees it again after reload.

### Wave 2: circles and enclosure

A2–E2. Draw a circle by clicking a centre and an edge point; the view lists the sites each circle
encloses; seeded circles render.

### Wave 3: other shapes

Polygons, rectangles, editing and deleting shapes. Scope is set by the analyst once wave 2 has
shipped.

## What the spec order has to settle (analyst gate)

None of these is a technical decision. If no SME answers them, a model will invent the answers:

- **What "correct" means for a drawn point.** A click lands within a pixel, and on a 3D globe a pixel
  is a different distance in metres depending on camera height and how near the edge of the globe the
  point is. The criterion needs a tolerance in business terms ("an endpoint within 1 km of where it
  was placed").
- **What a line is.** On a globe, the straight line between two points is either the shortest path
  over the surface (a geodesic) or the straight line on a flat map projection (a rhumb line). Cesium
  draws a geodesic by default. The spec decides; the tests check it.
- **Clicks off the land.** A click on space, or on the far side of the globe, places nothing. The spec
  says what the user sees.
- **Seeded expeditions.** Which ones, owned by whom, and whether a user can see other users'
  expeditions. That last point is a trust boundary, so the developer's security gate needs it.
- **Wave 2 questions, raised now:** whether a site on the edge of a circle counts as enclosed, and
  whether overlapping circles count a site once or twice.

## The package (order C1)

Learn from the Leaflet package's gaps; do not copy them:

- **It has no tests.** Its `test` script exits 1. C1's judge is a static fixture page inside the
  package (Cesium, the invented planet, known entities) and a suite run against it in the real-Chrome image.
- **Its `drawLine` adds a polyline through Leaflet's API.** That proves the assertion helpers, not
  that a user can draw. A test using it passes even when the app's drawing tool is broken. The Cesium
  package offers both:
  - **gestures**: `toScreen(lat, lng)` (null when the point is on the far side of the globe or
    off-screen), `clickAt(lat, lng)`, `flyTo`, `metersPerPixelAt(lat, lng)`, all through real
    `page.mouse` events on the canvas;
  - **state**: `getEntities()`, with positions in degrees, line endpoints, circle radius in metres and
    `saturdayMetadata`, plus `waitForSettled()`: tiles loaded, camera stopped, and no render pending.
    In 3D this is the wait most likely to make tests flaky, so it gets its own tests.
- **The app's drawing flow is not the package's job.** "Choose the line tool, click A, click B, save"
  is this app's UX, so it is a Saturday Flow in the app's test site (`draw-line.flow.ts`), built on
  the package's gestures. Package = map adapter; Flow = app behaviour; Page = the view's controls.
- **The seam is a contract the package publishes.** The app exposes
  `window.__SATURDAY_CESIUM__ = { viewer, Cesium }` (the tests need the Cesium module for
  `SceneTransforms` and `Cartographic`), and metadata on each entity. D1 implements the contract C1
  defines: the testability request from `03`, made up front.
- **A shared geometry core.** Distance, line and point-in-shape checks with Turf are the same for
  Leaflet and Cesium, and they are where correctness lives. Extract them into a pure package
  (`saturday-geo`), unit-tested in Node, used by both adapters. The tester decides at C1's design gate.

## Coverage of the map: heatmap and ML analyzer

Today both packages work on the DOM: the heatmap records clicks on elements and scans buttons, links
and inputs as "what could be tested"; the analyzer clusters those clicks and reports elements no test
came near. On a Cesium map every click lands on one `<canvas>`, and the reagent sites are entities,
not DOM elements, so both would report the map as a single element.

The useful version is a Cesium source for the heatmap: record each click as lat/lng (through the
package's `toScreen` inverse), and report entities from `getEntities()` as the interactables. The
analyzer then works unchanged and answers a real question: **which reagent sites and seeded shapes
did no test touch?** That is a cold-spot report for the map.

Finding the map on the page does not need ML: the seam gives its container and viewer directly.
Tracked in `docs/TODO.md`, "Maps"; after wave 1, not part of it.

## Judges

- **E, UI**: every scenario fails on at least one injected fault: a line saved with an endpoint
  moved, a seeded expedition at the wrong place, a site's metadata at the wrong position, the drawing
  tool ignoring the second click. Faults go in through a feature flag in the app or mock-API route
  interception (`03`).
- **F, API**: Sunday's mutations (`04`) on `/api/reagent-sites` and `/api/expeditions`.
- **The cross-check**: an E scenario draws a line in the UI and asserts what F's client reads back
  from the API. It is the only check that the shape is correct that does not depend on rendering.

## Phase 2 — a second target, to prove it is configuration

`testing-with-certs` has its own copy of the shop with a Leaflet `LocationsView`, three real services
with OpenAPI contracts in `contracts/openapi/`, and certificate login. Import A1's signed package there
as a new spec order (`source: import`, pinned to A1's digest), and run C1, D1, E1 and F1 against that
repo's `target.yaml`. Its SMEs review only what differs: authentication, which service owns sites,
the OpenAPI changes. If that needs more than a new `target.yaml` and a reviewed import, the factories
are less configurable than this design claims.

## Exit criteria for wave 1

1. A1, C1, D1, E1, F1 shipped as PRs. Each gate answered by the role in the table, and its
   corrections recorded.
2. Every E1 and F1 scenario failed at least one injected fault; the cross-check passes.
3. The Cesium package's own suite passes in the real-Chrome image with no network.
4. `05-pilot-decisions.md` has an entry for every by-hand stage, and each candidate has a decision:
   build it, keep watching, or drop it.
