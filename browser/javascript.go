package browser

import (
	"bytes"

	"github.com/PuerkitoBio/goquery"
	"github.com/robertkrimen/otto"
	//"github.com/robertkrimen/otto/parser" - Checkout the parser module, may be useful for intergration into surf
)

//# TODO: Review best examples
//https://github.com/emptyinterface/sq/blob/ae41c8755631006291f50f2b6eb42567e2ca050f/funcs.go
//https://github.com/beyondblog/wechat-spider/blob/aa10552c8eac551b58b2862bf61f33b94639ab2a/spider/spider.go

// Parser is a simple HTML parser
type Parser struct {
	ctx     *otto.Otto
	doc     *goquery.Document
	results map[string][]string
}

func registerVM(vm *otto.Otto) otto.Value {
	obj, _ := vm.Object("({})")

	obj.Set("newDocument", func(c otto.FunctionCall) otto.Value {
		str, _ := c.Argument(0).ToString()
		b := bytes.NewBufferString(str)
		doc, _ := goquery.NewDocumentFromReader(b)
		val, _ := c.Otto.ToValue(&doc)
		return val
	})

	return obj.Value()
}

// NewParser takes a url, does a HTTP get, builds a doc and return a parser or any error
func NewParser(url string, ctx *otto.Otto) (*Parser, error) {
	doc, err := goquery.NewDocument(url)
	if err != nil {
		return nil, err
	}

	p := &Parser{ctx: ctx, doc: doc, results: make(map[string][]string)}
	return p, nil
}

// EachFunc is a goquery iterator func
type EachFunc func(int, *goquery.Selection)

// Find takes a CSS selector and an EachFunc and applies them to the doc
func (p *Parser) Find(call otto.FunctionCall) otto.Value {
	if len(call.ArgumentList) < 3 ||
		!call.Argument(0).IsString() || !call.Argument(1).IsString() || !call.Argument(2).IsString() {
		return otto.FalseValue()
	}

	sel, el, key := call.Argument(0).String(), call.Argument(1).String(), call.Argument(2).String()

	p.doc.Find(sel).Each(p.get(el, key))
	return otto.TrueValue()
}

func (p *Parser) Results() otto.Value {
	val, err := p.ctx.ToValue(p.results)
	if err != nil {
		return otto.NullValue()
	}

	return val
}

func (p *Parser) get(el, key string) EachFunc {
	return func(i int, s *goquery.Selection) {
		if v, ok := s.Attr(el); ok {
			p.results[key] = append(p.results[key], v)
		}
	}
}
