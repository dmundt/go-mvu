# Admin dashboard

Interactive learning example, not a production administration system.

```powershell
go run .\examples\admin
```

Open <http://localhost:8086/admin>.

## Try the workflow

1. Page through the directory, then search for `Ada` or filter by role.
2. Choose **Edit** beside a member.
3. Submit a one-character name to see validation and retained values.
4. Save a valid name or role.
5. Choose **Refresh overview** to update the summary independently.
6. Turn off JavaScript and repeat. Links and forms still work.
7. Change appearance using the header controls.

The directory and editor share one replacement region. Overview uses a
second independent region and deliberately remains a snapshot until refreshed.
There is no polling, custom browser script, or hidden cross-region update.
Filtering and paging use real URLs and preserve query parameters.

## Keeping application code small

`Model.directory` prepares visible members, totals, and page bounds once per
workspace render. `directoryNavigation` gives the filter and pagination one
explicit history and request-ordering policy; `Pagination.BaseURL` preserves
the filters without a page-URL callback. Edit and refresh actions keep their
own policies.

Role choices and name limits are application-owned facts shared by seed data,
validation, and controls where relevant. `saveMember` owns copy-before-write
updates, retained invalid forms, and page clamping. These are ordinary Go
helpers, not another framework layer.

## What this demonstrates

| Concern | Implementation |
| --- | --- |
| Typed state transitions | `app.go`: messages and pure `Update` |
| Snapshot safety | Copy member slice before edits |
| Route/UI consistency | `actions.go`: shared action definitions |
| Explicit validation | Server checks names, roles, and record IDs |
| Reusable UI | `view.go`: shell, cards, stats, tables, forms, pagination |
| Partial HTML updates | Separate overview and workspace roots |
| Native fallback | GET navigation; POST/redirect/GET for edits |
| Request safety | Session-bound CSRF, body limits, deadlines, security headers |
| Lifecycle | Loopback listener and graceful shutdown |

## Boundaries

There is **no authentication or authorization**. Roles are sample data, not
access-control decisions. Never expose this handler as a real admin system.
Each visitor gets a separate in-memory directory; edits do not affect other
visitors. Changes and generated session secrets disappear on restart.

All names and addresses are synthetic. For login and durable storage, use the
[production example](../production/) and [production guide](../../docs/guides/production.md).
