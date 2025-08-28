package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cshum/vipsgen/vips"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

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

type WriteNopCloser struct {
	io.Writer
}

func (w *WriteNopCloser) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w *WriteNopCloser) Close() error {
	return nil
}

var limiter chan struct{}

func handler(w http.ResponseWriter, r *http.Request) {
	handleStart := time.Now()

	path := r.URL.Path
	if path == "/healthz" {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	parts := strings.SplitN(path, "/", 3)

	path = parts[2]

	u, err := url.Parse(path)
	if err != nil {
		log.Print("Parse url", err)
		w.WriteHeader(503)
		return
	}

	opts := parseOptions(parts[1])
	iw := maxwidth
	ih := maxheight
	crop := vips.InterestingNone

	if wv, exist := opts["width"]; exist {
		if wv == "auto" || wv == "" {

		} else {
			iw, _ = strconv.Atoi(wv)
		}
	}

	if hv, exist := opts["height"]; exist {
		if hv == "auto" || hv == "" {

		} else {
			ih, _ = strconv.Atoi(hv)
		}
	}

	if fit, exist := opts["fit"]; exist {
		switch fit {
		case "crop":
			crop = vips.InterestingCentre
		}
	}

	var url string
	if u.Scheme == "" {
		url = r.Header.Get("X-Forwarded-Proto") + "://" + r.Header.Get("X-Forwarded-Host") + "/" + path
	} else {
		url = path
	}

	waitStart := time.Now()
	limiter <- struct{}{}
	defer func() { <-limiter }()
	waitElalpsed := time.Since(waitStart)

	defer func() {
		log.Println(opts, url, r.Header.Get("X-Request-ID"), time.Since(handleStart).Seconds(), waitElalpsed.Seconds())
	}()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Print(err)
		w.WriteHeader(503)
		return
	}
	req.Header.Set("X-Request-ID", r.Header.Get("X-Request-ID"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Print(err)
		w.WriteHeader(503)
		return
	}
	defer resp.Body.Close()

	source := vips.NewSource(resp.Body)
	defer source.Close()

	img, err := vips.NewThumbnailSource(source, iw, &vips.ThumbnailSourceOptions{
		Height:   ih,
		Size:     vips.SizeDown,
		Crop:     crop,
		NoRotate: true,
		FailOn:   vips.FailOnError,
	})
	if err != nil {
		log.Print(err)
		w.WriteHeader(503)
		return
	}
	defer img.Close()

	target := vips.NewTarget(&WriteNopCloser{w})

	if strings.Contains(r.Header.Get("Accept"), "image/webp") {
		w.Header().Set("Content-Type", "image/webp")
		w.Header().Set("Vary", "Accept")
		w.WriteHeader(http.StatusOK)
		img.WebpsaveTarget(target, nil)
	} else {
		switch img.Format() {
		case vips.ImageTypePng:
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			img.PngsaveTarget(target, nil)
		default:
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			img.JpegsaveTarget(target, nil)
		}
	}
}

func main() {
	flag.IntVar(&maxwidth, "max-width", 3840, "")
	flag.IntVar(&maxheight, "max-height", 2160, "")
	flag.Parse()

	limiter = make(chan struct{}, 10)

	vips.Startup(&vips.Config{
		MaxCacheFiles:    0,
		ReportLeaks:      true,
		ConcurrencyLevel: 8,
		MaxCacheMem:      0,
		MaxCacheSize:     0,
	})
	defer vips.Shutdown()

	http.ListenAndServe(":8080", http.HandlerFunc(handler))
}
