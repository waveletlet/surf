Surf
====
**This is a custom fork for testing experimental features that may become pull requests, you should look at the base repository.**

Surf is a Go (golang) library that implements a virtual web browser that you control programmatically.
Surf isn't just another Go solution for downloading content from the web. Surf is designed to behave
like web browser, and includes: cookie management, history, bookmarking, user agent spoofing
(with a nifty user agent builder), submitting forms, DOM selection and traversal via jQuery style
CSS selectors, scraping assets like images, stylesheets, and other features.

* [Installation](#installation)
* [General Usage](#quick-start)

### Installation
Import the library into your project.
`import "gopkg.in/lostinblue/surf"`


### Quick Start
```go
package main

import (
	"gopkg.in/lostinblue/surf"
	"fmt"
)

func main() {
	bow := surf.NewBrowser()
	err := bow.Open("http://golang.org")
	if err != nil {
		panic(err)
	}

	// Outputs: "The Go Programming Language"
	fmt.Println(bow.Title())
}
```

