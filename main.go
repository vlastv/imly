package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"net/http"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/cshum/imagor"
	"github.com/cshum/imagor/imagorpath"
	"github.com/cshum/imagor/loader/httploader"
	"github.com/cshum/imagor/processor/vipsprocessor"
	"github.com/cshum/vipsgen/vips"
)

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

func handleOk(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	return
}

func isNoopRequest(r *http.Request) bool {
	return r.Method == http.MethodGet && (r.URL.Path == "/healthcheck" || r.URL.Path == "/favicon.ico")
}

func noopHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isNoopRequest(r) {
			handleOk(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeBody(w http.ResponseWriter, r *http.Request, reader io.ReadCloser, size int64) {
	defer func() {
		_ = reader.Close()
	}()
	if size > 0 {
		// total size known, use io.Copy
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		if r.Method != http.MethodHead {
			_, _ = io.Copy(w, reader)
		}
	} else {
		// total size unknown, read all
		buf, _ := io.ReadAll(reader)
		w.Header().Set("Content-Length", strconv.Itoa(len(buf)))
		if r.Method != http.MethodHead {
			_, _ = w.Write(buf)
		}
	}
}

type Service struct {
	app *imagor.Imagor
}

func (s *Service) Startup(ctx context.Context) error {
	vips.Startup(&vips.Config{
		MaxCacheFiles:    0,
		MaxCacheMem:      0,
		MaxCacheSize:     0,
		ConcurrencyLevel: 1,
	})

	return nil
}

func (s *Service) Shutdown(ctx context.Context) error {
	vips.Shutdown()

	return nil
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	parts := strings.SplitN(r.URL.Path, "/", 3)

	opts := parseOptions(parts[1])
	path := parts[2]

	p := imagorpath.Params{
		Unsafe: true,
		Path:   r.URL.Path,
		Image:  path,
	}

	if wv, exist := opts["width"]; exist {
		if wv == "auto" || wv == "" {

		} else {
			iw, _ := strconv.Atoi(wv)
			p.Width = iw
		}
	}

	if hv, exist := opts["height"]; exist {
		if hv == "auto" || hv == "" {

		} else {
			ih, _ := strconv.Atoi(hv)
			p.Height = ih
		}
	}

	if fit, exist := opts["fit"]; exist {
		switch fit {
		case "crop":

		}
	} else {
		p.FitIn = true
	}

	blob, err := s.app.Do(r, p)

	if err != nil {
		if errors.Is(err, context.Canceled) {
			w.WriteHeader(499)
			return
		}

		w.WriteHeader(503)
		return
	}

	w.Header().Set("Content-Type", blob.ContentType())

	reader, size, _ := blob.NewReader()
	writeBody(w, r, reader, size)
}

type options struct {
	baseUrl   string
	maxWidth  int
	maxHeight int
}

func main() {
	opts := options{}
	flag.StringVar(&opts.baseUrl, "base-url", "", "")
	flag.IntVar(&opts.maxWidth, "max-width", 3840, "Max width")
	flag.IntVar(&opts.maxHeight, "max-height", 2160, "Max height")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	svc := &Service{
		app: imagor.New(
			imagor.WithUnsafe(true),
			imagor.WithDebug(true),
			imagor.WithLoaders(
				httploader.New(
					httploader.WithBaseURL(opts.baseUrl),
				),
			),
			imagor.WithProcessors(
				vipsprocessor.NewProcessor(
					vipsprocessor.WithMaxWidth(opts.maxWidth),
					vipsprocessor.WithMaxHeight(opts.maxHeight),
				),
			),
		),
	}
	svc.Startup(ctx)
	defer svc.Shutdown(ctx)

	srv := &http.Server{
		Addr:    ":8000",
		Handler: noopHandler(svc),
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}

		log.Println("Listen on " + srv.Addr)
	}()

	<-ctx.Done()
}
