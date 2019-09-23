// Package surf ensembles other packages into a usable browser.
package surf

import (
	"github.com/waveletlet/surf/agent"
	"github.com/waveletlet/surf/browser"
	"github.com/waveletlet/surf/jar"
)

var (
	// DefaultUserAgent is the global user agent value.
	DefaultUserAgent = agent.Create()

	// DefaultSendReferer is the global value for the AttributeSendReferer attribute.
	DefaultSendReferer = true

	// DefaultMetaRefreshHandling is the global value for the AttributeHandleRefresh attribute.
	DefaultMetaRefreshHandling = true

	// DefaultFollowRedirects is the global value for the AttributeFollowRedirects attribute.
	DefaultFollowRedirects = true

	// DefaultMaxHistoryLength is the global value for max history length.
	DefaultMaxHistoryLength = 0
)

// NewBrowser creates and returns a browser.Browser type.
func NewBrowser() browser.Browser {
	bow := browser.Browser{}
	bow = bow.SetUserAgent(DefaultUserAgent)
	bow = bow.SetState(&jar.State{})
	bow = bow.SetCookieJar(jar.NewMemoryCookies())
	bow = bow.SetBookmarksJar(jar.NewMemoryBookmarks())
	hist := jar.NewMemoryHistory()
	hist.SetMax(DefaultMaxHistoryLength)
	bow = bow.SetHistoryJar(hist)
	bow = bow.SetHeadersJar(jar.NewMemoryHeaders())
	bow = bow.SetAttributes(browser.AttributeMap{
		browser.SendReferer:         DefaultSendReferer,
		browser.MetaRefreshHandling: DefaultMetaRefreshHandling,
		browser.FollowRedirects:     DefaultFollowRedirects,
	})

	return bow
}
