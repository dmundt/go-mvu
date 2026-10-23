# Admin dashboard

Interactive learning example, not a production administration system.

```powershell
go run .\examples\admin
```

Open <http://localhost:8086/admin>.

## Try the workflow

1. Page through the directory, then search for `Ada` or filter by role.
2. Choose **Quick edit** beside a member to open a native non-modal popover;
   **Edit** opens the equivalent full-page workflow.
3. Submit a one-character name to see a 422 fragment, retained values, and
   accessible field errors without closing the popover.
4. Save a valid name or role. The workspace is replaced, removing the open
   popover and installing a fresh closed shell. Status updates out of band;
   focus moves to **Apply filters**, which remains available even if the edited
   member no longer matches the filter.
5. Choose **Refresh overview** to update the summary independently.
6. Turn off JavaScript and repeat. Links and forms still work.
7. Change appearance using the header controls.

The directory and popover shell share one workspace region. Edit loads only
the inner content; invalid submissions replace that content, not the shell.
A successful save replaces the workspace and updates the separate live status
region with `hx-swap-oob`. Overview deliberately remains a snapshot until
refreshed. There is no polling or custom browser script. Filtering and paging
use real URLs and preserve query parameters.

## Overlay contract

Quick edit is **non-modal**, not a blocking modal: native `popover="auto"`
provides showing, Escape, and light dismissal; the background is not inert.
The shell uses `role="dialog"` with an accessible name, never `aria-modal`.
Its rendering composes existing semantic components; it is not a new
framework modal service or a change to `shared.Dialog`.

The shell exists before a request. Native `popovertarget` shows it while HTMX
loads member content. Cancel hides it immediately and requests draft cleanup.
Escape/light dismissal only changes native visibility; it does not erase a
server draft. A late content response cannot reopen the shell, and another
edit reloads current member data. Requests inherit one `hx-sync` replacement
scope so a slower edit cannot replace a newer selection or explicit cancel.
Save buttons are disabled during submission; this is a UX guard, not a durable
duplicate-suppression guarantee.

Browsers without native Popover API support should use full-page **Edit**;
no polyfill or client script is included. With JavaScript disabled, **Edit**
and ordinary form submission preserve the complete editing workflow.

Browser checks cover keyboard opening/submission, native focus return on Escape,
focus after save, computed styles, validation, repeated open, background access,
delayed responses, cancellation, history, and JavaScript-disabled submission.

## What this demonstrates

| Concern | Implementation |
| --- | --- |
| Typed state transitions | `app.go`: messages and pure `Update` |
| Snapshot safety | Copy member slice before edits |
| Route/UI consistency | `actions.go`: shared action definitions |
| Explicit validation | Server checks names, roles, and record IDs |
| Reusable UI | `view.go`: shell, cards, stats, tables, forms, pagination |
| Partial HTML updates | Workspace replacement, inner editor swaps, OOB status, independent overview |
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
