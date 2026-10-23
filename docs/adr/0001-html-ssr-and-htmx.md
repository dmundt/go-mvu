# ADR-0001: Focus on HTML server-side rendering and HTMX

- Status: Accepted

## Context

go-mvu targets Go applications that render HTML on the server and use HTMX to
enhance ordinary browser navigation and form interactions. Earlier brief
proposals considered generic renderers for Markdown, PDF, JSON, terminal, and
other targets. Those options broaden the framework beyond its intended
problem.

## Decision

Build a deliberately narrow framework around `net/http`, server-side rendering,
HTML, MVU, and HTMX. HTMX is the browser-side enhancement dependency; all
application behavior, state transitions, validation, persistence, and HTML
generation remain server-side.

Do not add a SPA runtime, hydration, VDOM, JSON UI protocol, client-side
application state, custom application JavaScript, runtime `html/template`
dependency, reflection-based rendering, or generic non-HTML renderer.

## Consequences

- Components and views produce HTML for full-page responses or HTMX fragment
  swaps.
- Applications can retain ordinary HTTP navigation and progressively enhance
  interactions with HTMX.
- New abstractions must serve concrete HTML/HTMX use cases rather than
  hypothetical output targets.

