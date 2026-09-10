# Feature: Browse past runs from the console

The console records every scenario run it starts, but there is no way to look at them.
`POST /api/runs` starts one and `GET /api/runs/{runId}` fetches a single run by id — nothing
lists them. A user who has closed the tab has no route back to a run they started ten minutes
ago except guessing its id.

Add a run history: a listing endpoint on the console API, and a page in the console that shows
the runs and lets someone open the report for any of them.

## Why
Debugging a flaky suite means comparing this run against the last few. Today that comparison
is impossible without ids captured by hand at the moment each run started, which nobody does.
The reports themselves are already served under `/reports/`, so the missing piece is purely
the index that points at them.

## Acceptance criteria
- `GET /api/runs` returns previously recorded runs, most recent first
- The response is paged, and a caller can ask for the next page without re-reading what it
  already has. The run store will grow for as long as the console is up, so the endpoint must
  not return everything it holds
- A caller can narrow the listing to a single status, so "show me the failures" is one request
- Asking for a page beyond the end returns an empty page, not an error
- The console has a page listing the runs — id, status, when it started, how long it took —
  where each row links to that run's report
- A run still in progress is shown as such and has no report link
- Someone using the keyboard alone can move through the list and open a report
- The listing endpoint responds in under 200ms at p95 with 10,000 runs held in the store
- Listing runs never blocks a run that is starting or finishing

## Out of scope
- Persisting runs across a console restart — the store stays in memory
- Deleting or archiving old runs
- Any change to how reports themselves are generated or served

## Notes
`internal/runs/store.go` holds the runs and is read and written from several goroutines.
`internal/httpserver/router.go` registers the routes; `handlers.go` and `responses.go` hold
the handlers and their response shapes.
