package server

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/time/rate"
)

func (cms *CmsStruct) uncaughtErrorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				cms.Logger.Error("CRITICAL: Uncaught error.", "error", fmt.Sprintf("%s", err))
				cms.internalError(w, fmt.Errorf("%s", err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (cms *CmsStruct) headersMiddleware(next httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		h := w.Header()

		if cms.Config.HTTPSMode {
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "br") {
				w.WriteHeader(http.StatusNotAcceptable)
				fmt.Fprint(w, "Brotli support required for HTTPS mode.")
				return
			}
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		// Etags are handled by handlers.
		h.Set("Content-Encoding", "br")
		h.Set("Cache-Control", "public, no-cache, no-transform, must-revalidate, s-maxage=0")
		h.Set("Content-Type", "text/html; charset=utf-8")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Vary", "Accept-Encoding")

		next(w, r, ps)
	}
}

func (cms *CmsStruct) rateLimitMiddleware(next http.Handler) http.Handler {
	type client struct {
		limitter  *rate.Limiter
		lastSince time.Time
	}
	clients := map[string]*client{}
	mutexClients := sync.Mutex{}
	go func() {
		time.Sleep(time.Minute)
		mutexClients.Lock()
		for ip, client := range clients {
			if time.Since(client.lastSince) >= time.Duration(float64(time.Second)*float64(cms.Config.RateLimit.Burst)/cms.Config.RateLimit.Rps) {
				delete(clients, ip)
			}
		}
		mutexClients.Unlock()
	}()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			cms.internalError(w, err)
			return
		}
		mutexClients.Lock()
		if _, found := clients[ip]; !found {
			clients[ip] = &client{rate.NewLimiter(rate.Limit(cms.Config.RateLimit.Rps), cms.Config.RateLimit.Burst), time.Now()}
		} else {
			temp := clients[ip]
			temp.lastSince = time.Now()
		}
		if !clients[ip].limitter.Allow() {
			mutexClients.Unlock()
			cms.tooManyRequests(w)
			return
		}
		mutexClients.Unlock()
		next.ServeHTTP(w, r)
	})
}
