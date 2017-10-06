package browser

import (
	"io"
	"net/http"
	"net/url"
)

// AssetType describes a type of page asset, such as an image or stylesheet.
type AssetType uint16

const (
	// LinkAsset describes a *Link asset.
	LinkAsset AssetType = iota

	// ImageAsset describes an *Image asset.
	ImageAsset

	// StylesheetAsset describes a *Stylesheet asset.
	StylesheetAsset

	// ScriptAsset describes a *Script asset.
	ScriptAsset
)

// AsyncDownloadResult has the results of an asynchronous download.
type AsyncDownloadResult struct {
	// Asset is a pointer to the Downloadable asset that was downloaded.
	Asset Downloadable

	// Writer where the asset data was written.
	Writer io.Writer

	// Size is the number of bytes written to the io.Writer.
	Size int64

	// Error contains any error that occurred during the download or nil.
	Error error
}

// AsyncDownloadChannel is a channel upon which the results of an async download
// are passed.
type AsyncDownloadChannel chan *AsyncDownloadResult

// Assetable represents a page asset, such as an image or stylesheet.
type Assetable interface {
	// URL returns the asset URL.
	URL() *url.URL

	// ID returns the asset ID or an empty string when not available.
	ID() string

	// Type describes the type of asset.
	AssetType() AssetType
}

// Asset implements Assetable.
type Asset struct {
	// ID is the value of the id attribute if available.
	ID string

	// URL is the asset URL.
	URL *url.URL

	// Type describes the type of asset.
	Type AssetType
}

// URL returns the asset URL.
func (at *Asset) URL() *url.URL {
	return at.URL
}

// ID returns the asset ID or an empty string when not available.
func (at *Asset) ID() string {
	return at.ID
}

// Type returns the asset type.
func (at *Asset) AssetType() AssetType {
	return at.Type
}

// Downloadable represents an asset that may be downloaded.
type Downloadable interface {
	Assetable

	// Download writes the contents of the element to the given writer.
	//
	// Returns the number of bytes written.
	Download(out io.Writer) (int64, error)

	// DownloadAsync downloads the contents of the element asynchronously.
	//
	// An instance of AsyncDownloadResult will be sent down the given channel
	// when the download is complete.
	DownloadAsync(out io.Writer, ch AsyncDownloadChannel)
}

// DownloadableAsset is an asset that may be downloaded.
type DownloadableAsset struct {
	Asset
}

// Download writes the asset to the given io.Writer type.
func (at *DownloadableAsset) Download(out io.Writer) (int64, error) {
	return DownloadAsset(at, out)
}

// DownloadAsync downloads the asset asynchronously.
func (at *DownloadableAsset) DownloadAsync(out io.Writer, ch AsyncDownloadChannel) {
	DownloadAssetAsync(at, out, ch)
}

// Link stores the properties of a page link.
type Link struct {
	Asset

	// Text is the text appearing between the opening and closing anchor tag.
	Text string
}

// NewLinkAsset creates and returns a new *Link type.
func NewLinkAsset(u *url.URL, id, text string) *Link {
	return &Link{
		Asset: Asset{
			URL:  u,
			ID:   id,
			Type: LinkAsset,
		},
		Text: text,
	}
}

// Image stores the properties of an image.
type Image struct {
	DownloadableAsset

	// Alt is the value of the image alt attribute if available.
	Alt string

	// Title is the value of the image title attribute if available.
	Title string
}

// NewImageAsset creates and returns a new *Image type.
func NewImageAsset(url *url.URL, id, alt, title string) *Image {
	return &Image{
		DownloadableAsset: DownloadableAsset{
			Asset: Asset{
				URL:  url,
				ID:   id,
				Type: ImageAsset,
			},
		},
		Alt:   alt,
		Title: title,
	}
}

// Stylesheet stores the properties of a linked stylesheet.
type Stylesheet struct {
	DownloadableAsset

	// Media is the value of the media attribute. Defaults to "all" when not specified.
	Media string

	// Type is the value of the type attribute. Defaults to "text/css" when not specified.
	Type string
}

// NewStylesheetAsset creates and returns a new *Stylesheet type.
func NewStylesheetAsset(url *url.URL, id, media, typ string) *Stylesheet {
	return &Stylesheet{
		DownloadableAsset: DownloadableAsset{
			Asset: Asset{
				URL:  url,
				Type: StylesheetAsset,
				ID:   id,
			},
		},
		Media: media,
		Type:  typ,
	}
}

// Script stores the properties of a linked script.
type Script struct {
	DownloadableAsset

	// Type is the value of the type attribute. Defaults to "text/javascript" when not specified.
	Type string
}

// NewScriptAsset creates and returns a new *Script type.
func NewScriptAsset(url *url.URL, id, typ string) *Script {
	return &Script{
		DownloadableAsset: DownloadableAsset{
			Asset: Asset{
				URL:  url,
				Type: ScriptAsset,
				ID:   id,
			},
		},
		Type: typ,
	}
}

// DownloadAsset copies a remote file to the given writer.
//# TODO: Should int64 be returned?
func DownloadAsset(asset Downloadable, out io.Writer) (int64, error) {
	//# TODO: out may be nil, this needs a check
	resp, err := http.Get(asset.URL().String())
	if err != nil {
		return 0, err
	}
	//# TODO: check if Body is nil before closing,
	// since web requests are not always successful and the nil pointer
	// error will be confusing to debug.
	if resp.Body != nil {
		defer resp.Body.Close()

		return io.Copy(out, resp.Body)
	}
	return 0, nil
}

// DownloadAssetAsync downloads an asset asynchronously and notifies the given channel
// when the download is complete.
func DownloadAssetAsync(asset Downloadable, out io.Writer, c AsyncDownloadChannel) {
	go func() {
		results := &AsyncDownloadResult{Asset: asset, Writer: out}
		size, err := DownloadAsset(asset, out)
		if err != nil {
			results.Error = err
		} else {
			results.Size = size
		}
		c <- results
	}()
}
