// Package surf ensembles other packages into a usable browser.
package surf

import (
	"github.com/lostinblue/surf/browser"
)

// NewBrowser creates and returns a *browser.Browser type.
func NewBrowser() *browser.Browser {
	bow := &browser.Browser{}
	//# TODO: All this initializing feels like it should be inside Browser init() function
	bow.Initialize()
	return bow
}
