package httpfly

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

// RoutePrefix is the prefix for all routes.
var RoutePrefix = "/api"

// MiddlewareFunc defines the type for middleware functions.
type MiddlewareFunc func(rb *RequestBody, response http.ResponseWriter, request *http.Request)

var middlewares []MiddlewareFunc

// AddMiddleware adds a new middleware to the handler.
func AddMiddleware(f MiddlewareFunc) {
	middlewares = append(middlewares, f)
}

// HandlingMode represents the mode for request handling.
type HandlingMode bool

const (
	// Sync mode indicates synchronous request handling.
	Sync HandlingMode = false
	// Async mode indicates asynchronous request handling.
	Async HandlingMode = true
)

// AuthRequire defines whether authentication is required for a route.
type AuthRequire bool

const (
	// UseAuth indicates authentication is required.
	UseAuth AuthRequire = true
	// NoAuth indicates authentication is not required.
	NoAuth AuthRequire = false
)

// RouteInfo defines information about a route.
type RouteInfo struct {
	Mode         HandlingMode
	Endpoint     string
	Method       RequestMethod
	AuthRequired bool
	HandlerF     Handler
}

// Routers
var routes []*RouteInfo

// MapGet maps a GET route.
func MapGet(mode HandlingMode, path string, auth AuthRequire, f func(r *RequestBody)) {
	routes = append(routes, &RouteInfo{mode, RoutePrefix + path, get, bool(auth), f})
}

// MapPost maps a POST route.
func MapPost(mode HandlingMode, path string, auth AuthRequire, f func(r *RequestBody)) {
	routes = append(routes, &RouteInfo{mode, RoutePrefix + path, post, bool(auth), f})
}

// MapPut maps a PUT route.
func MapPut(mode HandlingMode, path string, auth AuthRequire, f func(r *RequestBody)) {
	routes = append(routes, &RouteInfo{mode, RoutePrefix + path, put, bool(auth), f})
}

// MapDelete maps a DELETE route.
func MapDelete(mode HandlingMode, path string, auth AuthRequire, f func(r *RequestBody)) {
	routes = append(routes, &RouteInfo{mode, RoutePrefix + path, delete, bool(auth), f})
}

// StartHTTPServer starts the HTTP server.
func StartHTTPServer(listen string) {
	http.HandleFunc("/", handle)
	http.ListenAndServe(listen, nil)
}

// StartHTTPServerTLS starts the HTTPS server.
func StartHTTPServerTLS(listen string, certFile string, keyFile string) {
	http.HandleFunc("/", handle)
	http.ListenAndServeTLS(listen, certFile, keyFile, nil)
}

// Variables for response time optimizations
var avarageResMs int64 = 0
var totalRequest int64 = 0
var resetAt int64 = 9223372036054775807
var startOptimizeAt int64 = 1000

// It performs runtime optimizations as needed when the handled request count exceeds a specified threshold. Be mindful of setting this value appropriately: if set too low, it may lead to unnecessary operations, while setting it too high may delay optimization efforts

func SetOptimizationThreshold(threshold int64) {
	startOptimizeAt = threshold
}

var (
	totalRequestMutex sync.Mutex
	avarageResMsMutex sync.Mutex
)

func handle(resw http.ResponseWriter, req *http.Request) {
	for _, v := range routes {
		if req.URL.Path == v.Endpoint {
			if req.Method != string(v.Method) {
				resw.WriteHeader(http.StatusNotFound)
				return
			}

			rqbody := &RequestBody{}

			params, err := extractParams(req.URL.Path, v.Endpoint)

			if err != nil {
				resw.WriteHeader(http.StatusBadRequest)
				resw.Write([]byte(err.Error()))
				return
			}

			rqbody.Params = Parameters(params)

			req.Body.Read(rqbody.JsonData)

			for _, m := range middlewares {
				m(rqbody, resw, req)
			}

			t := time.Now()

			if v.Mode == Async {
				go v.HandlerF(rqbody)
			} else {
				v.HandlerF(rqbody)
			}

			totalRequestMutex.Lock()
			averageResMsMutex.Lock()

			// Start to runtime optimizations
			elapsed := time.Since(t).Microseconds()

			if totalRequest == resetAt || avarageResMs == resetAt {
				totalRequest = 0
				avarageResMs = 0
			}

			if totalRequest > startOptimizeAt && elapsed > avarageResMs && v.Mode != Async {
				// If sync request takes more time than avarage make it async.
				v.Mode = Async
			}

			totalRequest++
			avarageResMs += elapsed / totalRequest

			averageResMsMutex.Unlock()
			totalRequestMutex.Unlock()
			return
		}
	}

	// If no matching route is found, return 404
	resw.WriteHeader(http.StatusNotFound)
}

// extractParams extracts parameters from the URL path.
func extractParams(path string, locPath string) (map[string][]byte, error) {
	result := map[string][]byte{}

	p := paramExtractAlg(path)
	lp := paramExtractAlg(locPath)

	if len(p) != len(lp) {
		return nil, errors.New("invalid URL params")
	}

	for i := 0; i < len(p); i++ {
		result[lp[i]] = []byte(p[i])
	}

	return result, nil
}

// paramExtractAlg extracts parameters from a path.
func paramExtractAlg(input string) []string {
	var res []string
	var buildStr []rune

	for _, c := range input {
		switch c {
		case '{':
			buildStr = nil
		case '}':
			res = append(res, string(buildStr))
		default:
			buildStr = append(buildStr, c)
		}
	}

	return res
}

// RequestMethod represents an HTTP request method.
type RequestMethod string

const (
	get    RequestMethod = "GET"
	post   RequestMethod = "POST"
	put    RequestMethod = "PUT"
	delete RequestMethod = "DELETE"
)

// Parameters represents parameters extracted from a request.
type Parameters map[string][]byte

// RequestBody represents the request body.
type RequestBody struct {
	JsonData []byte
	Params   Parameters
	Claims   map[string]string
}

// Handler defines the type for request handlers.
type Handler func(r *RequestBody)
