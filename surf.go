// Package surf ensembles other packages into a usable browser.
package surf

import (
	"github.com/lostinblue/surf/agent"
	"github.com/lostinblue/surf/browser"
	"github.com/lostinblue/surf/jar"
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

// NewBrowser creates and returns a *browser.Browser type.
func NewBrowser() *browser.Browser {
	bow := &browser.Browser{}
	//# TODO: All this initializing feels like it should be inside Browser init() function
	bow.Initialize()
	return bow
}
