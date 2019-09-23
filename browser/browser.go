package browser

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/proxy"

	"github.com/PuerkitoBio/goquery"
	"github.com/waveletlet/surf/errors"
	"github.com/waveletlet/surf/jar"
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
	SetUserAgent(ua string) Browser

	// SetAttribute sets a browser instruction attribute.
	SetAttribute(a Attribute, v bool) Browser

	// SetAttributes is used to set all the browser attributes.
	SetAttributes(a AttributeMap) Browser

	// SetState sets the init browser state.
	SetState(sj *jar.State) Browser

	// State returns the browser state.
	State() *jar.State

	// SetBookmarksJar sets the bookmarks jar the browser uses.
	SetBookmarksJar(bj jar.BookmarksJar) Browser

	// BookmarksJar returns the bookmarks jar the browser uses.
	BookmarksJar() jar.BookmarksJar

	// SetCookieJar is used to set the cookie jar the browser uses.
	SetCookieJar(cj http.CookieJar) Browser

	// CookieJar returns the cookie jar the browser uses.
	CookieJar() http.CookieJar

	// SetHistoryJar is used to set the history jar the browser uses.
	SetHistoryJar(hj jar.History) Browser

	// HistoryJar returns the history jar the browser uses.
	HistoryJar() jar.History

	// SetHeadersJar sets the headers the browser sends with each request.
	SetHeadersJar(h http.Header) Browser

	// SetTimeout sets the timeout for requests.
	SetTimeout(t time.Duration) Browser

	// SetTransport sets the http library transport mechanism for each request.
	SetTransport(rt http.RoundTripper) Browser

	// Set a proxy URL.
	SetProxy(u string) (Browser, error)

	// AddRequestHeader adds a header the browser sends with each request.
	AddRequestHeader(name, value string) Browser

	// Open requests the given URL using the GET method.
	Open(url string) (Browser, error)

	// Open requests the given URL using the HEAD method.
	Head(url string) (Browser, error)

	// OpenForm appends the data values to the given URL and sends a GET request.
	OpenForm(url string, data url.Values) (Browser, error)

	// OpenBookmark calls Get() with the URL for the bookmark with the given name.
	OpenBookmark(name string) (Browser, error)

	// Post requests the given URL using the POST method.
	Post(url string, contentType string, body io.Reader) (Browser, error)

	// PostForm requests the given URL using the POST method with the given data.
	PostForm(url string, data url.Values) (Browser, error)

	// PostMultipart requests the given URL using the POST method with the given data using multipart/form-data format.
	PostMultipart(u string, fields url.Values, files FileSet) (Browser, error)

	// Back loads the previously requested page.
	Back() (Browser, bool)

	// Reload duplicates the last successful request.
	Reload() (Browser, error)

	// Bookmark saves the page URL in the bookmarks with the given name.
	Bookmark(name string) (Browser, error)

	// Click clicks on the page element matched by the given expression.
	Click(expr string) (Browser, error)

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

	// ResolveUrl returns an absolute URL for a possibly relative URL.
	ResolveUrl(u *url.URL) *url.URL

	// ResolveStringUrl works just like ResolveUrl, but the argument and return value are strings.
	ResolveStringUrl(u string) (string, error)

	// Download writes the contents of the document to the given writer.
	Download(o io.Writer) (int64, error)

	// Url returns the page URL as a string.
	Url() *url.URL

	// StatusCode returns the response status code.
	StatusCode() int

	// Title returns the page title.
	Title() string

	// ResponseHeaders returns the page headers.
	ResponseHeaders() http.Header

	// Body returns the page body as a string of html.
	Body() string

	// Dom returns the inner *goquery.Selection.
	Dom() *goquery.Selection

	// Find returns the dom selections matching the given expression.
	Find(expr string) *goquery.Selection
}

// Browser is the default Browser implementation.
type Browser struct {
	// HTTP client
	client *http.Client

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

	// body of the current page.
	body []byte
}

// buildClient instantiates the *http.Client used by the browser
func (bow Browser) buildClient() *http.Client {
	return &http.Client{
		CheckRedirect: bow.shouldRedirect,
	}
}

func (bow Browser) rebuildClient() {
	// call this to make sure browser object is updated before calling client.Do()
	// are there any other client functions that need an updated browser before
	// running?
	bow.client.CheckRedirect = bow.shouldRedirect
}

// Open requests the given URL using the GET method.
func (bow Browser) Open(u string) (Browser, error) {
	ur, err := url.Parse(u)
	if err != nil {
		return bow, err
	}
	return bow.httpGET(ur, nil)
}

// Head requests the given URL using the HEAD method.
func (bow Browser) Head(u string) (Browser, error) {
	ur, err := url.Parse(u)
	if err != nil {
		return bow, err
	}
	return bow.httpHEAD(ur, nil)
}

// OpenForm appends the data values to the given URL and sends a GET request.
func (bow Browser) OpenForm(u string, data url.Values) (Browser, error) {
	ul, err := url.Parse(u)
	if err != nil {
		return bow, err
	}
	ul.RawQuery = data.Encode()

	return bow.Open(ul.String())
}

// OpenBookmark calls Open() with the URL for the bookmark with the given name.
func (bow Browser) OpenBookmark(name string) (Browser, error) {
	url, err := bow.bookmarks.Read(name)
	if err != nil {
		return bow, err
	}
	return bow.Open(url)
}

// Post requests the given URL using the POST method.
func (bow Browser) Post(u string, contentType string, body io.Reader) (Browser, error) {
	ur, err := url.Parse(u)
	if err != nil {
		return bow, err
	}
	return bow.httpPOST(ur, bow.Url(), contentType, body)
}

// PostForm requests the given URL using the POST method with the given data.
func (bow Browser) PostForm(u string, data url.Values) (Browser, error) {
	return bow.Post(u, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

// PostMultipart requests the given URL using the POST method with the given data using multipart/form-data format.
func (bow Browser) PostMultipart(u string, fields url.Values, files FileSet) (Browser, error) {
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
			return bow, err
		}
		if file.data != nil {
			_, err = io.Copy(fw, file.data)
			if err != nil {
				return bow, err
			}
		}
	}
	err := writer.Close()
	if err != nil {
		return bow, err

	}
	return bow.Post(u, writer.FormDataContentType(), body)
}

// Put requests the given URL using the PUT method.
func (bow Browser) Put(u string, contentType string, body io.Reader) (Browser, error) {
	ur, err := url.Parse(u)
	if err != nil {
		return bow, err
	}
	return bow.httpPUT(ur, bow.Url(), contentType, body)
}

// Delete requests the given URL using the DELETE method.
func (bow Browser) Delete(u string) (Browser, error) {
	ur, err := url.Parse(u)
	if err != nil {
		return bow, err
	}
	return bow.httpDELETE(ur, nil)
}

// Back loads the previously requested page.
//
// Returns a boolean value indicating whether a previous page existed, and was
// successfully loaded.
func (bow Browser) Back() (Browser, bool) {
	if bow.history.Len() > 1 {
		bow.state = bow.history.Pop()
		return bow, true
	}
	return bow, false
}

// Reload duplicates the last successful request.
func (bow Browser) Reload() (Browser, error) {
	if bow.state.Request != nil {
		return bow.httpRequest(bow.state.Request)
	}
	return bow, errors.NewPageNotLoaded("Cannot reload, the previous request failed.")
}

// Bookmark saves the page URL in the bookmarks with the given name.
// TODO how will this work stateless? bookmarks.Save() needs to give back the
// browser?
func (bow Browser) Bookmark(name string) (Browser, error) {
	return bow, bow.bookmarks.Save(name, bow.ResolveUrl(bow.Url()).String()) // NOT SANE ACTUALLY
}

// Click clicks on the page element matched by the given expression.
//
// Currently this is only useful for click on links, which will cause the browser
// to load the page pointed at by the link. Future versions of Surf may support
// JavaScript and clicking on elements will fire the click event.
func (bow Browser) Click(expr string) (Browser, error) {
	sel := bow.Find(expr)
	if sel.Length() == 0 {
		return bow, errors.NewElementNotFound(
			"Element not found matching expr '%s'.", expr)
	}
	if !sel.Is("a") {
		return bow, errors.NewElementNotFound(
			"Expr '%s' must match an anchor tag.", expr)
	}

	href, err := bow.attrToResolvedUrl("href", sel)
	if err != nil {
		return bow, err
	}

	return bow.httpGET(href, bow.Url())
}

// Form returns the form in the current page that matches the given expr.
func (bow Browser) Form(expr string) (Submittable, error) {
	sel := bow.Find(expr)
	if sel.Length() == 0 {
		return nil, errors.NewElementNotFound(
			"Form not found matching expr '%s'.", expr)
	}
	if !sel.Is("form") {
		return nil, errors.NewElementNotFound(
			"Expr '%s' does not match a form tag.", expr)
	}

	return NewForm(bow, sel), nil
}

// Forms returns an array of every form in the page.
func (bow Browser) Forms() []Submittable {
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
func (bow Browser) Links() []*Link {
	links := make([]*Link, 0, InitialAssetsSliceSize)
	bow.Find("a").Each(func(_ int, s *goquery.Selection) {
		href, err := bow.attrToResolvedUrl("href", s)
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
func (bow Browser) Images() []*Image {
	images := make([]*Image, 0, InitialAssetsSliceSize)
	bow.Find("img").Each(func(_ int, s *goquery.Selection) {
		src, err := bow.attrToResolvedUrl("src", s)
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
func (bow Browser) Stylesheets() []*Stylesheet {
	stylesheets := make([]*Stylesheet, 0, InitialAssetsSliceSize)
	bow.Find("link").Each(func(_ int, s *goquery.Selection) {
		rel, ok := s.Attr("rel")
		if ok && rel == "stylesheet" {
			href, err := bow.attrToResolvedUrl("href", s)
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
func (bow Browser) Scripts() []*Script {
	scripts := make([]*Script, 0, InitialAssetsSliceSize)
	bow.Find("script").Each(func(_ int, s *goquery.Selection) {
		src, err := bow.attrToResolvedUrl("src", s)
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
func (bow Browser) SiteCookies() []*http.Cookie {
	if bow.client == nil {
		bow.client = bow.buildClient()
	}
	return bow.client.Jar.Cookies(bow.Url())
}

// SetState sets the browser state.
func (bow Browser) SetState(sj *jar.State) Browser {
	bow.state = sj
	return bow
}

// State returns the browser state.
func (bow Browser) State() *jar.State {
	return bow.state
}

// SetCookieJar is used to set the cookie jar the browser uses.
func (bow Browser) SetCookieJar(cj http.CookieJar) Browser {
	if bow.client == nil {
		bow.client = bow.buildClient()
	}
	bow.client.Jar = cj
	return bow
}

// CookieJar returns the cookie jar the browser uses.
func (bow Browser) CookieJar() http.CookieJar {
	if bow.client == nil {
		bow.client = bow.buildClient()
	}
	return bow.client.Jar
}

// SetUserAgent sets the user agent.
func (bow Browser) SetUserAgent(userAgent string) Browser {
	bow.userAgent = userAgent
	return bow
}

// SetAttribute sets a browser instruction attribute.
func (bow Browser) SetAttribute(a Attribute, v bool) Browser {
	bow.attributes[a] = v
	return bow
}

// SetAttributes is used to set all the browser attributes.
func (bow Browser) SetAttributes(a AttributeMap) Browser {
	bow.attributes = a
	return bow
}

// SetBookmarksJar sets the bookmarks jar the browser uses.
func (bow Browser) SetBookmarksJar(bj jar.BookmarksJar) Browser {
	bow.bookmarks = bj
	return bow
}

// BookmarksJar returns the bookmarks jar the browser uses.
func (bow Browser) BookmarksJar() jar.BookmarksJar {
	return bow.bookmarks
}

// SetHistoryJar is used to set the history jar the browser uses.
func (bow Browser) SetHistoryJar(hj jar.History) Browser {
	bow.history = hj
	return bow
}

// HistoryJar returns the history jar the browser uses.
func (bow Browser) HistoryJar() jar.History {
	return bow.history
}

// SetHeadersJar sets the headers the browser sends with each request.
func (bow Browser) SetHeadersJar(h http.Header) Browser {
	bow.headers = h
	return bow
}

// SetTimeout sets the timeout for requests.
func (bow Browser) SetTimeout(t time.Duration) Browser {
	if bow.client == nil {
		bow.client = bow.buildClient()
	}
	bow.client.Timeout = t
	return bow
}

// SetTransport sets the http library transport mechanism for each request.
func (bow Browser) SetTransport(rt http.RoundTripper) {
	if bow.client == nil {
		bow.client = bow.buildClient()
	}
	bow.client.Transport = rt
}

// Set a proxy url
func (bow Browser) SetProxy(u string) error {
	_url, err := url.Parse(u)
	if err != nil {
		return err
	}
	dialer, err := proxy.FromURL(_url, proxy.Direct)
	if err != nil {
		return err
	}
	bow.SetTransport(&http.Transport{Dial: dialer.Dial})

	return nil
}

// AddRequestHeader sets a header the browser sends with each request.
func (bow Browser) AddRequestHeader(name, value string) Browser {
	//TODO test working?
	fmt.Println("bow.headers")
	fmt.Println(bow.headers)
	// header.Set and header.Add are NOT the same thing, probably want to add a
	// separate function for setting vs adding
	bow.headers.Set(name, value)
	fmt.Println(bow.headers)
	return bow
}

// DelRequestHeader deletes a header so the browser will not send it with future requests.
func (bow Browser) DelRequestHeader(name string) Browser {
	fmt.Println("bow.headers")
	//TODO test working?
	fmt.Println(bow.headers)
	bow.headers.Del(name)
	fmt.Println(bow.headers)
	return bow
}

// ResolveUrl returns an absolute URL for a possibly relative URL.
func (bow Browser) ResolveUrl(u *url.URL) *url.URL {
	return bow.Url().ResolveReference(u)
}

// ResolveStringUrl works just like ResolveUrl, but the argument and return value are strings.
func (bow Browser) ResolveStringUrl(u string) (string, error) {
	pu, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	pu = bow.Url().ResolveReference(pu)
	return pu.String(), nil
}

// TODO bad function name, bow.body is already downloaded. Should be called
// "Save" or "WriteTo" or something
//
// Download writes the contents of the document to the given writer.
func (bow Browser) Download(o io.Writer) (int64, error) {
	buff := bytes.NewBuffer(bow.body)
	return io.Copy(o, buff)
}

// Url returns the page URL as a string.
func (bow Browser) Url() *url.URL {
	if bow.state.Response == nil {
		// there is a possibility that we issued a request, but for
		// whatever reason the request failed.
		if bow.state.Request != nil {
			return bow.state.Request.URL
		}
		return nil
	}

	return bow.state.Response.Request.URL
}

// StatusCode returns the response status code.
func (bow Browser) StatusCode() int {
	return bow.state.Response.StatusCode
}

// Title returns the page title.
func (bow Browser) Title() string {
	return bow.state.Dom.Find("title").Text()
}

// ResponseHeaders returns the page headers.
func (bow Browser) ResponseHeaders() http.Header {
	return bow.state.Response.Header
}

// Body returns the page body as a string of html.
func (bow Browser) Body() string {
	body, _ := bow.state.Dom.Find("body").Html()
	return body
}

// Dom returns the inner *goquery.Selection.
func (bow Browser) Dom() *goquery.Selection {
	return bow.state.Dom.First()
}

// Find returns the dom selections matching the given expression.
func (bow Browser) Find(expr string) *goquery.Selection {
	return bow.state.Dom.Find(expr)
}

// buildRequest creates and returns a *http.Request type.
// Sets any headers that need to be sent with the request.
func (bow Browser) buildRequest(method, url string, ref *url.URL, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
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
func (bow Browser) httpGET(u *url.URL, ref *url.URL) (Browser, error) {
	req, err := bow.buildRequest("GET", u.String(), ref, nil)
	if err != nil {
		return bow, err
	}
	return bow.httpRequest(req)
}

// httpHEAD makes an HTTP HEAD request for the given URL.
// When via is not nil, and AttributeSendReferer is true, the Referer header will
// be set to ref.
func (bow Browser) httpHEAD(u *url.URL, ref *url.URL) (Browser, error) {
	req, err := bow.buildRequest("HEAD", u.String(), ref, nil)
	if err != nil {
		return bow, err
	}
	return bow.httpRequest(req)
}

// httpPOST makes an HTTP POST request for the given URL.
// When via is not nil, and AttributeSendReferer is true, the Referer header will
// be set to ref.
func (bow Browser) httpPOST(u *url.URL, ref *url.URL, contentType string, body io.Reader) (Browser, error) {
	req, err := bow.buildRequest("POST", u.String(), ref, body)
	if err != nil {
		return bow, err
	}
	req.Header.Set("Content-Type", contentType)

	return bow.httpRequest(req)
}

// httpPUT makes an HTTP PUT request for the given URL.
// When via is not nil, and AttributeSendReferer is true, the Referer header will
// be set to ref.
func (bow Browser) httpPUT(u *url.URL, ref *url.URL, contentType string, body io.Reader) (Browser, error) {
	req, err := bow.buildRequest("PUT", u.String(), ref, body)
	if err != nil {
		return bow, err
	}
	req.Header.Set("Content-Type", contentType)

	return bow.httpRequest(req)
}

// httpDELETE makes an HTTP DELETE request for the given URL.
// When via is not nil, and AttributeSendReferer is true, the Referer header will
// be set to ref.
func (bow Browser) httpDELETE(u *url.URL, ref *url.URL) (Browser, error) {
	req, err := bow.buildRequest("DELETE", u.String(), ref, nil)
	if err != nil {
		return bow, err
	}
	return bow.httpRequest(req)
}

// send uses the given *http.Request to make an HTTP request.
func (bow Browser) httpRequest(req *http.Request) (Browser, error) {
	if bow.client == nil {
		bow.client = bow.buildClient()
	}
	bow.preSend()

	bow.rebuildClient()
	resp, err := bow.client.Do(req)

	if err != nil {
		return bow, err
	}
	defer resp.Body.Close()

	var reader io.Reader
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return bow, err
		}
	case "deflate":
		reader = flate.NewReader(resp.Body)

	default:
		reader = resp.Body
	}

	bow.body, err = ioutil.ReadAll(reader)
	if err != nil {
		return bow, err
	}

	buff := bytes.NewBuffer(bow.body)
	dom, err := goquery.NewDocumentFromReader(buff)
	if err != nil {
		return bow, err
	}

	// TODO check these functions are stateless
	bow.history.Push(bow.state)
	bow.state = jar.NewHistoryState(req, resp, dom)
	bow.postSend()

	return bow, nil
}

// preSend sets browser state before sending a request.
func (bow Browser) preSend() Browser {
	if bow.refresh != nil {
		bow.refresh.Stop()
	}
	return bow
}

// postSend sets browser state after sending a request.
func (bow Browser) postSend() Browser {
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
	return bow
}

// shouldRedirect is used as the value to http.Client.CheckRedirect.
func (bow Browser) shouldRedirect(req *http.Request, _ []*http.Request) error {
	if bow.attributes[FollowRedirects] {
		return nil
	}
	return errors.NewLocation(
		"Redirects are disabled. Cannot follow '%s'.", req.URL.String())
}

// attributeToUrl reads an attribute from an element and returns a url.
func (bow Browser) attrToResolvedUrl(name string, sel *goquery.Selection) (*url.URL, error) {
	src, ok := sel.Attr(name)
	if !ok {
		return nil, errors.NewAttributeNotFound(
			"Attribute '%s' not found.", name)
	}
	ur, err := url.Parse(src)
	if err != nil {
		return nil, err
	}

	return bow.ResolveUrl(ur), nil
}

// attributeOrDefault reads an attribute and returns it or the default value when it's empty.
func (bow Browser) attrOrDefault(name, def string, sel *goquery.Selection) string {
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
