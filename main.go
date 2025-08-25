package main

import (
	"bytes"
	"flag"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/reuseport"
)

var maxwidth int
var maxheight int

func normalizeOption(o string) string {
	switch o {
	case "w":
		return "width"
	case "h":
		return "height"
	default:
		return o
	}
}

func parseOptions(s string) map[string]string {
	pairs := strings.Split(s, ",")
	options := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		data := strings.SplitN(pair, "=", 2)
		options[normalizeOption(data[0])] = data[1]
	}

	return options
}

func handler(ctx *fasthttp.RequestCtx) {
	defer func() {
		log.Println(string(ctx.URI().PathOriginal()))
	}()

	path := string(ctx.URI().PathOriginal())
	if len(ctx.URI().QueryString()) > 0 {
		path += "?" + string(ctx.URI().QueryString())
	}
	parts := strings.SplitN(path, "/", 3)

	path = parts[2]

	u, err := url.Parse(path)
	if err != nil {
		log.Print(err)
		ctx.SetStatusCode(503)
		return
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()

	if u.Scheme == "" {
		req.SetHostBytes(ctx.Request.Header.Peek(fasthttp.HeaderXForwardedHost))
		req.URI().SetSchemeBytes(ctx.Request.Header.Peek(fasthttp.HeaderXForwardedProto))
		req.URI().SetPath(path)
	} else {
		req.SetRequestURI(u.String())
	}

	if err := fasthttp.Do(req, resp); err != nil {
		log.Print(err)
	}
	fasthttp.ReleaseRequest(req)

	img, err := vips.NewImageFromBuffer(resp.Body())
	fasthttp.ReleaseResponse(resp)
	if err != nil {
		log.Print(err)
		ctx.SetStatusCode(503)
		return
	}
	defer img.Close()

	opts := parseOptions(parts[1])
	w := maxwidth
	h := maxheight
	crop := vips.InterestingNone

	if wv, exist := opts["width"]; exist {
		if wv == "auto" {

		} else {
			w, _ = strconv.Atoi(wv)
		}
	}

	if hv, exist := opts["height"]; exist {
		if hv == "auto" {

		} else {
			h, _ = strconv.Atoi(hv)
		}
	}

	if fit, exist := opts["fit"]; exist {
		switch fit {
		case "crop":
			crop = vips.InterestingCentre
		}
	}

	img.ThumbnailWithSize(w, h, crop, vips.SizeDown)

	var params *vips.ExportParams

	if bytes.Contains(ctx.Request.Header.Peek("Accept"), []byte("image/webp")) {
		params = vips.NewDefaultWEBPExportParams()
		ctx.SetContentType("image/webp")
		ctx.Response.Header.Set("Vary", "Accept")
	} else {
		switch img.Format() {
		case vips.ImageTypePNG:
			params = vips.NewDefaultPNGExportParams()
			ctx.SetContentType("image/png")
		default:
			params = vips.NewDefaultJPEGExportParams()
			ctx.SetContentType("image/jpeg")
		}
	}
	params.StripMetadata = true

	b, _, err := img.Export(params)
	if err != nil {
		log.Print(err)
		ctx.SetStatusCode(503)
	}
	ctx.Write(b)
}

func main() {
	flag.IntVar(&maxwidth, "max-width", 3840, "")
	flag.IntVar(&maxheight, "max-height", 2160, "")
	flag.Parse()

	vips.LoggingSettings(nil, vips.LogLevelMessage)

	vips.Startup(nil)
	defer vips.Shutdown()

	ln, err := reuseport.Listen("tcp4", ":8080")
	if err != nil {
		log.Fatalln(err)
	}

	fasthttp.Serve(ln, handler)
}
