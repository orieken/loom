# Pilot decision log

Every choice in the pilot that would be made again on the next feature. Keep it to what was decided,
not what was done. The last column is the point: a decision made the same way twice is a candidate
to stop making by hand.

**Candidate kinds**: **skill** (a procedure an agent follows) · **tool** (deterministic code: a
lint, a generator, a `loom` command, a package helper) · **rule** (a line in `shared/rules/` or a
contract) · **template** (a file the factory copies) · **none**.

**Status**: `watching` (seen once) · `build` (seen twice or obviously general; make it) · `built`
(with a link) · `dropped` (with why).

## How to add an entry

Append a row after each by-hand stage. If the same decision comes up again, do not add a second row:
add the stage to "Seen in" and move it to `build`.

| # | Decision | Why | Seen in | Candidate | Status |
|---|---|---|---|---|---|
| D-1 | Specs, scenarios and tests live in the repo of the code they describe; the framework holds only plans, agents and contracts | Their reviewers are that product's SMEs, and they change when the product changes | design | rule (`01`) | build |
| D-2 | How to build, start and check a target is `target.yaml`, owned by the target; an order only names a target | Configurable per team without changing orders or plans | design | template + `loom intake check` rule | build |
| D-3 | Every gate has exactly one owning role, named in the order | "SMEs review their chunk" only holds if someone is named per chunk | design | rule (intake v2) | build |
| D-4 | A missing seam (test id, window hook, fixture) becomes a testability order to the app's factory, and the test order parks until it ships | Test factories never edit product code (gate #6) | design | tool (emit draft order) | watching |
| D-5 | Map packages expose two APIs: gestures through real mouse events, and state reads; never a helper that draws through the map library's API | The Leaflet `drawLine` passes tests when the drawing tool is broken | design | rule (`03` reviewer) | watching |
| D-6 | The app's UX for a map action is a Saturday Flow in the app's test site; the package is the map adapter only | Keeps the package reusable across apps | design | rule + `generate_flow` template | watching |
| D-7 | The seam contract (`window.__SATURDAY_<LIB>__`, entity metadata) is defined by the package, and implemented by the app before tests are written | Turns a testability discovery into a design input | design | template (seam contract doc per package) | watching |
| D-8 | No test reaches the network; maps use local imagery | The factory's containers have no egress, and a flaky tile server is not a test result | design | rule + `target.yaml` `egress: none` check | build |
| D-9 | A drawn shape is checked against the API, not only against the rendered map | The only check of correctness independent of rendering | design | rule (`03`/`04` cross-check) | watching |
| D-10 | A tolerance in a criterion is stated in business units (km, %), never pixels | Pixels-to-metres changes with zoom and camera | design | rule (`spec-critic` check) | watching |
| D-11 | Shapes ship one wave at a time; a later wave adds to the earlier spec package, never edits it | Keeps pinned digests valid for orders already built on them | design | rule (`02`) | watching |
| D-13 | The globe is an invented planet: a perfect sphere with a declared radius and one equirectangular image | Keeps it local and accountless, and a sphere makes Cesium and Turf agree exactly; WGS84 and Turf's sphere differ by up to about 0.5% | design | rule (`saturday-playwright-cesium` fixture) | watching |
| D-14 | Geometry helpers take the planet's radius from the seam, never Earth's constant | A made-up planet may not be Earth-sized | design | rule | watching |
| D-15 | Refactors of golden code are orders whose acceptance criteria are "the existing suites pass unchanged" | The existing scenarios are the characterization tests | golden orders | template (refactor order) | watching |
| D-16 | A pre-ship check is added only after the order that creates it merges, and only once it has run green end to end from a fresh install | Otherwise every order fails a check that does not exist yet, or one that cannot pass yet (`ui-suite` waits on `golden-workspace-packages`) | factory setup, ui-suite-real-chrome ship | rule | build |
| D-17 | A pre-manifest install is updated by taking over exactly the files loom would skip as "not installed by loom", then installing per platform | A plain install silently leaves most files at the old version; deleting whole directories loses platform detection | saturday update | tool (`scripts/update-framework.sh`; candidate for `loom update`) | build |
| D-18 | Before promoting an order, check that every file it names is committed: `git ls-files` on its scope and pinned paths | The first order named images in a gitignored folder; the worktree had none of them | first saturday run | tool (`loom intake check` warns on scope globs matching no tracked file) | build |
| D-19 | An order's `scope.paths` must include where its own acceptance criteria put new files | The first order required a new test but did not list `tests/**` | first saturday run | rule (`loom intake check` cannot see it; spec-critic can) | watching |
| D-20 | One runner image per framework, layered: a pinned, signature-checked, non-root `base` with real browsers, Xvfb and an opt-in viewer; project needs (certificates, toolchains) as targets or layers on top | Both repos had built their own; one shared base means one place to keep browsers current | runner image | template (`containers/saturday-runner`) | watching |
| D-21 | A container's entrypoint ends in `exec "$@"`; never supervisord as PID 1 for a test run | Otherwise the test command's exit code is lost and a pre-ship check cannot fail | runner image | rule | watching |
| D-22 | Long build work (image builds, package locks under emulation) runs outside a factory stage until stages can be given longer limits | The first developer stage was killed at 30 minutes mid-build | first saturday run | none until L3.78 | watching |
| D-23 | Software WebGL in the runner image is headed Chrome under Xvfb with `--use-gl=angle --use-angle=swiftshader`, in `runner-check` and in every suite's Chrome project; the design's Mesa llvmpipe claim was wrong | The first debian build got a null `webgl2` context with no GL args, `--use-gl=desktop` and `--use-angle=gl`; only ANGLE SwiftShader rendered | runner image | template (Playwright `chrome` project launch args) | watching |
| D-24 | Anything a suite or check writes into the mounted checkout (reports, `test-results/`, `evidence/`) is gitignored, never tracked | Otherwise the commit gate lists run artifacts outside `scope.paths`, and a human has to clean them up by hand | ui-suite-real-chrome commit | rule + `loom intake check` warns on tracked artifact paths | watching |
| D-25 | Before removing a parked run's worktree, commit its uncommitted work to a `salvage/<order>` branch and point the narrowed order at that commit | The worktree removal in the re-claim workaround deletes the only copy; the re-run reused the drafts instead of rewriting them | ui-suite-real-chrome re-run | tool (`loom factory rerun` could do it; order `reclaim-onto-claimed-commit`) | watching |
| D-12 | A stage becomes a factory plan once it has been run by hand twice and its decisions have settled | Build from evidence, not from the design | design | none | — |

## Open (decide during wave 1)

- Geodesic or rhumb-line straight line (analyst).
- Whether users see each other's expeditions (analyst, then developer at the security gate).
- Whether to extract `saturday-geo` now (tester, at C1's design gate).
