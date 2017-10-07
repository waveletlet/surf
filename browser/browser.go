package browser

import (
	"bytes"
	"io"
	"io/ioutil"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/proxy"
	"github.com/robertkrimen/otto"
	"github.com/PuerkitoBio/goquery"
	"github.com/lostinblue/surf/errors"
	"github.com/lostinblue/surf/jar"
)

// Attribute represents a Browser capability.
type Attribute int

// AttributeMap represents a map of Attribute values.
type AttributeMap map[Attribute]bool

// File represents a input type file, that includes the fileName and a io.reader
type File struct {
	fileName string
	data     io.Reader
}

// FileSet represents a map of files used to port multipart
type FileSet map[string]*File

const (
	// SendReferer instructs a Browser to send the Referer header.
	SendReferer Attribute = iota

	// MetaRefreshHandling instructs a Browser to handle the refresh meta tag.
	MetaRefreshHandling

	// FollowRedirects instructs a Browser to follow Location headers.
	FollowRedirects
)

// InitialAssetsSliceSize is the initial size when allocating a slice of page
// assets. Increasing this size may lead to a very small performance increase
// when downloading assets from a page with a lot of assets.
var InitialAssetsSliceSize = 20

// Browsable represents an HTTP web browser.
type Browsable interface {
	// SetUserAgent sets the user agent.
	SetUserAgent(ua string)

	// Get UserAgent value
	UserAgent() string

	// SetAttribute sets a browser instruction attribute.
	SetAttribute(a Attribute, v bool)

	// SetAttributes is used to set all the browser attributes.
	SetAttributes(a AttributeMap)

	// Get Attribute value from Attribute
	Attribute(a Attribute) bool

	// SetState sets the init browser state.
	SetState(sj *jar.State)

	// State returns the browser state.
	State() *jar.State

	// SetBookmarksJar sets the bookmarks jar the browser uses.
	SetBookmarksJar(bj jar.BookmarksJar)

	// BookmarksJar returns the bookmarks jar the browser uses.
	BookmarksJar() jar.BookmarksJar

	// SetCookieJar is used to set the cookie jar the browser uses.
	SetCookieJar(cj http.CookieJar)

	// CookieJar returns the cookie jar the browser uses.
	CookieJar() http.CookieJar

	// SetHistoryJar is used to set the history jar the browser uses.
	SetHistoryJar(hj jar.History)

	// HistoryJar returns the history jar the browser uses.
	HistoryJar() jar.History

	// SetHeadersJar sets the headers the browser sends with each request.
	SetHeadersJar(h http.Header)

	// SetTimeout sets the timeout for requests.
	SetTimeout(t time.Duration)

	// Get Timeout returns the value of the private timeout attribute
	Timeout() time.Duration

	// SetTransport sets the http library transport mechanism for each request.
	SetTransport(rt http.RoundTripper)

	// SetProxy sets the proxy used by the browser
	SetProxy(u string) (err error)

	// Get Proxy returns the proxy details
	Proxy() string

	// AddRequestHeader adds a header the browser sends with each request.
	AddRequestHeader(name, value string)

	// GET requests the given URL using the GET method.
	GET(u string) error

	// HEAD requests the given URL using the HEAD method.
	HEAD(u string) error

	// POST requests the given URL using the POST method.
	POST(u string, contentType string, body io.Reader) error

	// GETForm appends the data values to the given URL and sends a GET request.
	GETForm(u string, data url.Values) error

	// OpenBookmark calls Get() with the URL for the bookmark with the given name.
	OpenBookmark(name string) error

	// PostForm requests the given URL using the POST method with the given data.
	POSTForm(u string, data url.Values) error

	// PostMultipart requests the given URL using the POST method with the given data using multipart/form-data format.
	POSTMultipart(u string, fields url.Values, files FileSet) error

	// Back loads the previously requested page.
	Back() bool

	// Reload duplicates the last successful request.
	Reload() error

	// Bookmark saves the page URL in the bookmarks with the given name.
	Bookmark(name string) error

	// Click clicks on the page element matched by the given expression.
	Click(expr string) error

	// Form returns the form in the current page that matches the given expr.
	Form(expr string) (Submittable, error)

	// Forms returns an array of every form in the page.
	Forms() []Submittable

	// Links returns an array of every link found in the page.
	Links() []*Link

	// Images returns an array of every image found in the page.
	Images() []*Image

	// Stylesheets returns an array of every stylesheet linked to the document.
	Stylesheets() []*Stylesheet

	// Scripts returns an array of every script linked to the document.
	Scripts() []*Script

	// SiteCookies returns the cookies for the current site.
	SiteCookies() []*http.Cookie

	// ResolveURL returns an absolute URL for a possibly relative URL.
	ResolveURL(u *url.URL) *url.URL

	// ResolveStringURL works just like ResolveURL, but the argument and return value are strings.
	ResolveStringURL(u string) (string, error)

	// Download writes the contents of the document to the given writer.
	Download(o io.Writer) (int64, error)

	// URL returns the page URL as a string.
	URL() *url.URL

	// StatusCode returns the response status code.
	StatusCode() int

	// Title returns the page title.
	Title() string

	// ResponseHeaders returns the page headers.
	ResponseHeaders() http.Header

	// RequestHeaders return the client request headers.
	RequestHeaders() http.Header

	// HTML returns the HTML tag as a string of html.
	HTML() string

	// Body returns the page body as a string of html.
	Body() string

	// DOM returns the inner *goquery.Document.
	DOM() *goquery.Document

	// Find returns the dom selections matching the given expression.
	Find(expr string) *goquery.Selection

	// NewTab returns a new Browser instance and inherit the configuration
	// Read more: https://github.com/headzoo/surf/issues/23
	NewTab() (bow *Browser)

	// NewJavaScriptVM returns a new Otto Javascript VM.
	NewJavaScriptVM()
}

// Browser is the default Browser implementation.
type Browser struct {
	// HTTP client
	client *http.Client

	// Javascript VM
	javaScriptVM *otto.Otto

	// state is the current browser state.
	state *jar.State

	// userAgent is the User-Agent header value sent with requests.
	userAgent string

	// bookmarks stores the saved bookmarks.
	bookmarks jar.BookmarksJar

	// history stores the visited pages.
	history jar.History

	// headers are additional headers to send with each request.
	headers http.Header

	// attributes is the set browser attributes.
	attributes AttributeMap

	// refresh is a timer used to meta refresh pages.
	refresh *time.Timer

	// all html of the current page.
	html []byte

	// body of the current page.
	body []byte

	// timeout of the request
	timeout time.Duration
}

// buildClient instanciates the *http.Client used by the browser
func (bow *Browser) buildClient() *http.Client {
	return &http.Client{
		CheckRedirect: bow.shouldRedirect,
	}
}

// GET requests the given URL using the GET method.
func (bow *Browser) GET(u string) error {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return err
	}
	return bow.httpGET(parsedURL, nil)
}

// HEAD requests the given URL using the HEAD method.
func (bow *Browser) HEAD(u string) error {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return err
	}
	return bow.httpHEAD(parsedURL, nil)
}

// GETForm appends the data values to the given URL and sends a GET request.
func (bow *Browser) GETForm(u string, data url.Values) error {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return err
	}
	parsedURL.RawQuery = data.Encode()
	return bow.GET(parsedURL.String())
}

// OpenBookmark calls GET() with the URL for the bookmark with the given name.
func (bow *Browser) OpenBookmark(name string) error {
	bookmarkURL, err := bow.bookmarks.Read(name)
	if err != nil {
		return err
	}
	return bow.GET(bookmarkURL)
}

// POST requests the given URL using the POST method.
func (bow *Browser) POST(u string, contentType string, body io.Reader) error {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return err
	}
	return bow.httpPOST(parsedURL, bow.URL(), contentType, body)
}

// POSTForm requests the given URL using the POST method with the given data.
func (bow *Browser) POSTForm(u string, data url.Values) error {
	return bow.Post(u, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

// POSTMultipart requests the given URL using the POST method with the given data using multipart/form-data format.
func (bow *Browser) POSTMultipart(u string, fields url.Values, files FileSet) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for k, vs := range fields {
		for _, v := range vs {
			writer.WriteField(k, v)
		}
	}
	for k, file := range files {
		fw, err := writer.CreateFormFile(k, file.fileName)
		if err != nil {
			return err
		}
		if file.data != nil {
			_, err = io.Copy(fw, file.data)
			if err != nil {
				return err
			}
		}
	}
	err := writer.Close()
	if err != nil {
		return err

	}
	return bow.Post(u, writer.FormDataContentType(), body)
}

// Back loads the previously requested page.
//
// Returns a boolean value indicating whether a previous page existed, and was
// successfully loaded.
func (bow *Browser) Back() bool {
	if bow.history.Len() > 1 {
		bow.state = bow.history.Pop()
		return true
	}
	return false
}

// Reload duplicates the last successful request.
func (bow *Browser) Reload() error {
	if bow.state.Request != nil {
		return bow.httpRequest(bow.state.Request)
	}
	return errors.NewPageNotLoaded("Cannot reload, the previous request failed.")
}

// Bookmark saves the page URL in the bookmarks with the given name.
func (bow *Browser) Bookmark(name string) error {
	//# TODO: Resolve seems redundant when URL is only loaded upon succsesful page load
	return bow.bookmarks.Save(name, bow.ResolveURL(bow.URL()).String())
}

// Click clicks on the page element matched by the given expression.
//
// Currently this is only useful for click on links, which will cause the browser
// to load the page pointed at by the link. Future versions of Surf may support
// JavaScript and clicking on elements will fire the click event.
//# TODO: Implement Javascript clicking with otto
func (bow *Browser) Click(expr string) error {
	sel := bow.Find(expr)
	if sel.Length() == 0 {
		return errors.NewElementNotFound("Element not found matching expr '%s'.", expr)
	}
	if !sel.Is("a") {
		return errors.NewElementNotFound("Expr '%s' must match an anchor tag.", expr)
	}
	href, err := bow.attrToResolvedURL("href", sel)
	if err != nil {
		return err
	}

	return bow.httpGET(href, bow.URL())
}

// Form returns the form in the current page that matches the given expr.
func (bow *Browser) Form(expr string) (Submittable, error) {
	sel := bow.Find(expr)
	if sel.Length() == 0 {
		return nil, errors.NewElementNotFound("Form not found matching expr '%s'.", expr)
	}
	if !sel.Is("form") {
		return nil, errors.NewElementNotFound("Expr '%s' does not match a form tag.", expr)
	}
	return NewForm(bow, sel), nil
}

// Forms returns an array of every form in the page.
func (bow *Browser) Forms() []Submittable {
	sel := bow.Find("form")
	len := sel.Length()
	if len == 0 {
		return nil
	}

	forms := make([]Submittable, len)
	sel.Each(func(_ int, s *goquery.Selection) {
		forms = append(forms, NewForm(bow, s))
	})
	return forms
}

// Links returns an array of every link found in the page.
func (bow *Browser) Links() []*Link {
	links := make([]*Link, 0, InitialAssetsSliceSize)
	bow.Find("a").Each(func(_ int, s *goquery.Selection) {
		href, err := bow.attrToResolvedURL("href", s)
		if err == nil {
			links = append(links, NewLinkAsset(
				href,
				bow.attrOrDefault("id", "", s),
				s.Text(),
			))
		}
	})

	return links
}

// Images returns an array of every image found in the page.
func (bow *Browser) Images() []*Image {
	images := make([]*Image, 0, InitialAssetsSliceSize)
	bow.Find("img").Each(func(_ int, s *goquery.Selection) {
		src, err := bow.attrToResolvedURL("src", s)
		if err == nil {
			images = append(images, NewImageAsset(
				src,
				bow.attrOrDefault("id", "", s),
				bow.attrOrDefault("alt", "", s),
				bow.attrOrDefault("title", "", s),
			))
		}
	})

	return images
}

// Stylesheets returns an array of every stylesheet linked to the document.
func (bow *Browser) Stylesheets() []*Stylesheet {
	stylesheets := make([]*Stylesheet, 0, InitialAssetsSliceSize)
	bow.Find("link").Each(func(_ int, s *goquery.Selection) {
		rel, ok := s.Attr("rel")
		if ok && rel == "stylesheet" {
			href, err := bow.attrToResolvedURL("href", s)
			if err == nil {
				stylesheets = append(stylesheets, NewStylesheetAsset(
					href,
					bow.attrOrDefault("id", "", s),
					bow.attrOrDefault("media", "all", s),
					bow.attrOrDefault("type", "text/css", s),
				))
			}
		}
	})

	return stylesheets
}

// Scripts returns an array of every script linked to the document.
func (bow *Browser) Scripts() []*Script {
	//# TODO: Flag to download during Get so it can be processed
	//# TODO: Include inline JS, combine it into a single JS file
	scripts := make([]*Script, 0, InitialAssetsSliceSize)
	bow.Find("script").Each(func(_ int, s *goquery.Selection) {
		src, err := bow.attrToResolvedURL("src", s)
		if err == nil {
			scripts = append(scripts, NewScriptAsset(
						src,
						bow.attrOrDefault("id", "", s),
						bow.attrOrDefault("type", "text/javascript", s),
						))
		}
	})

	return scripts
}

// SiteCookies returns the cookies for the current site.
func (bow *Browser) SiteCookies() []*http.Cookie {
	if bow.client == nil {
		bow.client = bow.buildClient()
	}
	return bow.client.Jar.Cookies(bow.URL())
}

// SetState sets the browser state.
func (bow *Browser) SetState(sj *jar.State) {
	bow.state = sj
}

// State returns the browser state.
func (bow *Browser) State() *jar.State {
	return bow.state
}

// SetCookieJar is used to set the cookie jar the browser uses.
func (bow *Browser) SetCookieJar(cj http.CookieJar) {
	if bow.client == nil {
		bow.client = bow.buildClient()
	}
	bow.client.Jar = cj
}

// CookieJar returns the cookie jar the browser uses.
func (bow *Browser) CookieJar() http.CookieJar {
	if bow.client == nil {
		bow.client = bow.buildClient()
	}
	return bow.client.Jar
}

// SetUserAgent sets the user agent.
func (bow *Browser) SetUserAgent(userAgent string) {
	bow.userAgent = userAgent
}

func (bow *Browser) UserAgent() string {
	return bow.userAgent
}

// SetAttribute sets a browser instruction attribute.
func (bow *Browser) SetAttribute(a Attribute, v bool) {
	bow.attributes[a] = v
}

// SetAttributes is used to set all the browser attributes.
func (bow *Browser) SetAttributes(a AttributeMap) {
	bow.attributes = a
}

// Get Attribute value from Attribute
func (bow *Browser) Attribute(a Attribute) bool {
	return bow.attributes[a]
}

// SetBookmarksJar sets the bookmarks jar the browser uses.
func (bow *Browser) SetBookmarksJar(bj jar.BookmarksJar) {
	bow.bookmarks = bj
}

// BookmarksJar returns the bookmarks jar the browser uses.
func (bow *Browser) BookmarksJar() jar.BookmarksJar {
	return bow.bookmarks
}

// SetHistoryJar is used to set the history jar the browser uses.
func (bow *Browser) SetHistoryJar(hj jar.History) {
	bow.history = hj
}

// HistoryJar returns the history jar the browser uses.
func (bow *Browser) HistoryJar() jar.History {
	return bow.history
}

// SetHeadersJar sets the headers the browser sends with each request.
func (bow *Browser) SetHeadersJar(h http.Header) {
	bow.headers = h
}

// SetTransport sets the http library transport mechanism for each request.
// SetTimeout sets the timeout for requests.
func (bow *Browser) SetTimeout(t time.Duration) {
	bow.timeout = t
}

// Timeout gets the timeout value for requests.
func (bow *Browser) Timeout() time.Duration {
	return bow.timeout
}

// SetTransport sets the http library transport mechanism for each request.
func (bow *Browser) SetTransport(rt http.RoundTripper) {
	if bow.client == nil {
		bow.client = bow.buildClient()
	}
	bow.client.Transport = rt
}

// SetProxy allows the use of socks proxies, for example it can be used to connect using Tor.
func (bow *Browser) SetProxy(u string) (err error) {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return err
	}
	dialer, err := proxy.FromURL(parsedURL, proxy.Direct)
	if err != nil {
		return err
	}
	bow.SetTransport(&http.Transport{Dial: dialer.Dial})

	return err
}

func (bow *Browser) Proxy() string {
	return "not implemented"
}

// AddRequestHeader sets a header the browser sends with each request.
func (bow *Browser) AddRequestHeader(name, value string) {
	bow.headers.Set(name, value)
}

// DelRequestHeader deletes a header so the browser will not send it with future requests.
func (bow *Browser) DelRequestHeader(name string) {
	bow.headers.Del(name)
}

// ResolveURL returns an absolute URL for a possibly relative URL.
func (bow *Browser) ResolveURL(u *url.URL) *url.URL {
	return bow.URL().ResolveReference(u)
}

// ResolveStringURL works just like ResolveURL, but the argument and return value are strings.
func (bow *Browser) ResolveStringURL(u string) (string, error) {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	resolvedURL = bow.URL().ResolveReference(parsedURL)
	return resolvedURL.String(), nil
}

// Download writes the contents of the document to the given writer.
func (bow *Browser) Download(o io.Writer) (int64, error) {
	if o == nil {
		//# TODO: If o is nil, should either throw an error explaining the issue or just initialize it
		fmt.Fprintln(os.Stdout, "===== [o io.Writer is nil] =====\n")
	}
	//# TODO: Check body if nil
	buff := bytes.NewBuffer(bow.body)
	//fmt.Fprintln(os.Stdout, "===== [DUMP buff] =====\n", buff)
	//fmt.Fprintln(os.Stdout, "===== [DUMP bow.body] =====\n", bow.body)
	return io.Copy(o, buff)
	//return 0, errors.New("Failed to execut io.Copy(o, buff)")
}

// URL returns the page URL as a string.
func (bow *Browser) URL() *url.URL {
	if bow.state.Response == nil {
		//# TODO: Why not just return nil? Why check again?
		// there is a possibility that we issued a request, but for
		// whatever reason the request failed.
		//if bow.state.Request != nil {
		//	return bow.state.Request.URL
		//}
		return nil
	}

	return bow.state.Response.Request.URL
}

// StatusCode returns the response status code.
func (bow *Browser) StatusCode() int {
	// there is a possibility that we issued a request, but for
	// whatever reason the request failed.
	//# TODO: Since this is repeating, it may be necessary to add a specialized function, possibly in errors
	//  and this issue exists at least in 5 other spots in the codebase
	if bow.state.Response == nil {
		// Since this is not a pointer, it needs a value
		return 0
	}
	return bow.state.Response.StatusCode
}

// Title returns the page title.
func (bow *Browser) Title() string {
	return bow.state.Dom.Find("title").Text()
}

// ResponseHeaders returns the page headers.
func (bow *Browser) ResponseHeaders() http.Header {
	return bow.state.Response.Header
}

// RequestHeaders returns the client headers.
func (bow *Browser) RequestHeaders() http.Header {
	//TODO: Gather REQUEST headers and return them
	return nil
}

// HTML document as a string of html.
func (bow *Browser) HTML() string {
	html, _ := bow.state.Dom.First().Html()
	return html
}

// Body returns the page body as a string of html.
func (bow *Browser) Body() string {
	body, _ := bow.state.Dom.Find("body").Html()
	return body
}

// DOM returns the inner *goquery.Selection.
func (bow *Browser) DOM() *goquery.Document {
	return bow.state.Dom
}

// Find returns the dom selections matching the given expression.
func (bow *Browser) Find(expr string) *goquery.Selection {
	return bow.state.Dom.Find(expr)
}

func (bow *Browser) NewTab() (b *Browser) {
	b = &Browser{}
	//# TODO: Why use a pointer? and why this type of assignment?
	*b = *bow
	return b
}

func (bow *Browser) NewJavaScriptVM() {
	bow.javaScriptVM = otto.New()
}

// buildRequest creates and returns a *http.Request type.
// Sets any headers that need to be sent with the request.
func (bow *Browser) buildRequest(method, u string, ref *url.URL, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header = copyHeaders(bow.headers)

	if host := req.Header.Get("Host"); host != "" {
		req.Host = host
	}
	req.Header.Set("User-Agent", bow.userAgent)
	if bow.attributes[SendReferer] && ref != nil {
		req.Header.Set("Referer", ref.String())
	}
	if os.Getenv("SURF_DEBUG_HEADERS") != "" {
		d, _ := httputil.DumpRequest(req, false)
		fmt.Fprintln(os.Stderr, "===== [DUMP] =====\n", string(d))
	}

	return req, nil
}

func copyHeaders(h http.Header) http.Header {
	if h == nil {
		return nil
	}
	h2 := make(http.Header, len(h))
	for k, v := range h {
		h2[k] = v
	}
	return h2
}

// httpGET makes an HTTP GET request for the given URL.
// When via is not nil, and AttributeSendReferer is true, the Referer header will
// be set to ref.
//# TODO: Why does this exist, along with GET? Can this/should this be combined?
func (bow *Browser) httpGET(u *url.URL, ref *url.URL) error {
	req, err := bow.buildRequest("GET", u.String(), ref, nil)
	if err != nil {
		return err
	}
	return bow.httpRequest(req)
}

// httpHEAD makes an HTTP HEAD request for the given URL.
// When via is not nil, and AttributeSendReferer is true, the Referer header will
// be set to ref.
func (bow *Browser) httpHEAD(u *url.URL, ref *url.URL) error {
	req, err := bow.buildRequest("HEAD", u.String(), ref, nil)
	if err != nil {
		return err
	}
	return bow.httpRequest(req)
}

// httpPOST makes an HTTP POST request for the given URL.
// When via is not nil, and AttributeSendReferer is true, the Referer header will
// be set to ref.
func (bow *Browser) httpPOST(u *url.URL, ref *url.URL, contentType string, body io.Reader) error {
	req, err := bow.buildRequest("POST", u.String(), ref, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)

	return bow.httpRequest(req)
}

// httpRequest uses the given *http.Request to make an HTTP request.
func (bow *Browser) httpRequest(req *http.Request) error {
	if bow.client == nil {
		bow.client = bow.buildClient()
	}
	bow.preSend()
	resp, err := bow.client.Do(req)
	if err != nil {
		return err
	}
	// If resp.Body.Close() is called on an empty, it will throw a nil pointer error
	// if it is nil, then there is no reason to close it.
	if resp.Body != nil {
		defer resp.Body.Close()

		bow.body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		buff := bytes.NewBuffer(bow.body)
		dom, err := goquery.NewDocumentFromReader(buff)
		if err != nil {
			return err
		}

		bow.history.Push(bow.state)
		bow.state = jar.NewHistoryState(req, resp, dom)
		bow.postSend()
	}
	return nil
}

// preSend sets browser state before sending a request.
func (bow *Browser) preSend() {
	if bow.refresh != nil {
		bow.refresh.Stop()
	}
}

// postSend sets browser state after sending a request.
func (bow *Browser) postSend() {
	if isContentTypeHtml(bow.state.Response) && bow.attributes[MetaRefreshHandling] {
		sel := bow.Find("meta[http-equiv='refresh']")
		if sel.Length() > 0 {
			attr, ok := sel.Attr("content")
			if ok {
				dur, err := time.ParseDuration(attr + "s")
				if err == nil {
					bow.refresh = time.NewTimer(dur)
					go func() {
						<-bow.refresh.C
						bow.Reload()
					}()
				}
			}
		}
	}
}

// shouldRedirect is used as the value to http.Client.CheckRedirect.
func (bow *Browser) shouldRedirect(req *http.Request, _ []*http.Request) error {
	if bow.attributes[FollowRedirects] {
		req.Header.Set("User-Agent", bow.userAgent)
		return nil
	}
	return errors.NewLocation("Redirects are disabled. Cannot follow '%s'.", req.URL.String())
}

// attributeToURL reads an attribute from an element and returns a url.
func (bow *Browser) attrToResolvedURL(name string, sel *goquery.Selection) (*url.URL, error) {
	src, ok := sel.Attr(name)
	if !ok {
		return nil, errors.NewAttributeNotFound("Attribute '%s' not found.", name)
	}
	parsedURL, err := url.Parse(src)
	if err != nil {
		return nil, err
	}

	return bow.ResolveURL(parsedURL), nil
}

// attributeOrDefault reads an attribute and returns it or the default value when it's empty.
func (bow *Browser) attrOrDefault(name, def string, sel *goquery.Selection) string {
	a, ok := sel.Attr(name)
	if ok {
		return a
	}
	return def
}

// isContentTypeHtml returns true when the given response sent the "text/html" content type.
func isContentTypeHtml(res *http.Response) bool {
	if res != nil {
		ct := res.Header.Get("Content-Type")
		return ct == "" || strings.Contains(ct, "text/html")
	}
	return false
}
