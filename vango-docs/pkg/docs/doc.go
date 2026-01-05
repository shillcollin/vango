// Package docs provides a server-driven documentation framework for Vango.
//
// VangoDocs implements the MDG (Markdown + Go) architecture:
//   - Zero-bundle content: Markdown renders to Vango VNodes on the server
//   - Server-side search: In-memory index with instant results over WebSocket
//   - Live component hydration: Embed interactive Go components in Markdown
//
// Example usage:
//
//	import "github.com/vangojs/vango-docs/pkg/docs"
//
//	// In your route handler:
//	func DocPage(ctx vango.Ctx) vango.Component {
//	    page, _ := docs.GetPage(ctx.Param("slug"))
//	    return docs.Render(page)
//	}
package docs

// Version is the current version of VangoDocs.
const Version = "0.1.0"
