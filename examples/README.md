# Examples

Run commands from the repository root with Go 1.24 or newer.

| Example | Run | Shows |
| --- | --- | --- |
| [Hello World](hello/) | `go run .\examples\hello` | One page and heading; no runtime or HTMX |
| [Counter](counter/) | `go run .\examples\counter` | Simple buttons and one shared action definition |
| [Contact form](contact/) | `go run .\examples\contact` | Multiple fields, validation, errors, and no-JavaScript fallback |
| [Admin dashboard](admin/) | `go run .\examples\admin` | Filters, pagination, editable records, validation, and independent updates |
| [Users](users/) | `go run .\examples\users` | Tables, paging, and reusable components |
| [Themes](themes/) | `go run .\examples\themes` | Default theme and color modes |
| [Component gallery](components/) | `go run .\examples\components` | Reusable controls and layouts |
| [Durable app](durable/) | `go run .\examples\durable` | Stored data and a delivery queue |
| [Production app](production/) | See its README | Login, persistent storage, and deployment setup |

Start with hello, then counter and contact. Admin combines the pieces into
a larger interactive screen. Examples are for
learning; follow the [production guide](../docs/guides/production.md) before
putting one online.
