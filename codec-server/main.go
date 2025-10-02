package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	temporalconverters "go-codec-server"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
)

// JWKS configuration
const (
	jwksURL = "https://login.tmprl.cloud/.well-known/jwks.json"
)

// newCORSHTTPHandler wraps a HTTP handler with CORS support
func newCORSHTTPHandler(web string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", web)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Namespace")

		if r.Method == "OPTIONS" {
			return
		}

		next.ServeHTTP(w, r)
	})
}

// HTTP handler for codecs
func newPayloadCodecNamespacesHTTPHandler(encoders map[string][]converter.PayloadCodec) http.Handler {
	mux := http.NewServeMux()

	// Register namespace-specific routes
	for namespace, codecChain := range encoders {
		fmt.Printf("Registering routes for namespace: %s\n", namespace)
		ns := namespace
		chain := codecChain

		// Create manual encode handler
		mux.HandleFunc("/"+ns+"/encode", func(w http.ResponseWriter, r *http.Request) {
			handleCodecRequest(w, r, ns, chain, true)
		})

		// Create manual decode handler
		mux.HandleFunc("/"+ns+"/decode", func(w http.ResponseWriter, r *http.Request) {
			handleCodecRequest(w, r, ns, chain, false)
		})
	}

	// Handle requests with X-Namespace header
	mux.HandleFunc("/encode", func(w http.ResponseWriter, r *http.Request) {
		namespace := r.Header.Get("X-Namespace")
		if namespace == "" {
			namespace = "default"
		}
		if codecChain, ok := encoders[namespace]; ok {
			handleCodecRequest(w, r, namespace, codecChain, true)
		} else {
			fmt.Printf("No codec chain found for namespace: %s (available: %v)\n", namespace, getMapKeys(encoders))
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		}
	})

	mux.HandleFunc("/decode", func(w http.ResponseWriter, r *http.Request) {
		namespace := r.Header.Get("X-Namespace")
		if namespace == "" {
			namespace = "default"
		}
		if codecChain, ok := encoders[namespace]; ok {
			handleCodecRequest(w, r, namespace, codecChain, false)
		} else {
			fmt.Printf("No codec chain found for namespace: %s (available: %v)\n", namespace, getMapKeys(encoders))
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		}
	})

	return mux
}

// Helper function to get map keys
// Used to log the available namespace key in case of
// encode or decode errors
func getMapKeys(m map[string][]converter.PayloadCodec) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Always make a network call to the jwks endpoint
func validateJWTWithJWKS(tokenString string) (jwt.Token, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Fresh keys on every request
	keySet, err := jwk.Fetch(ctx, jwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}

	return jwt.Parse([]byte(tokenString), jwt.WithKeySet(keySet))
}

// Main handler for encode/decode requests with JWT auth
func handleCodecRequest(w http.ResponseWriter, r *http.Request, namespace string, codecChain []converter.PayloadCodec, isEncode bool) {
	operation := map[bool]string{true: "encode", false: "decode"}[isEncode]

	// Check JWT authorization
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		fmt.Printf("Missing Authorization header for %s operation on namespace %s\n", operation, namespace)
		http.Error(w, "Authorization header required", http.StatusUnauthorized)
		return
	}

	// Log the auth header (truncated for security)
	truncatedAuth := authHeader
	if len(authHeader) > 50 {
		truncatedAuth = authHeader[:47] + "..."
	}
	fmt.Printf("Processing %s request for namespace %s with auth: %s\n", operation, namespace, truncatedAuth)

	// Extract token from "Bearer <token>"
	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		fmt.Printf("Invalid authorization format for %s operation on namespace %s\n", operation, namespace)
		http.Error(w, "Invalid authorization format. Use 'Bearer <token>'", http.StatusUnauthorized)
		return
	}

	// Log the JWT token (truncated for security)
	jwtToken := tokenParts[1]
	truncatedToken := jwtToken
	if len(jwtToken) > 50 {
		truncatedToken = jwtToken[:47] + "..."
	}
	fmt.Printf("JWT token received: %s\n", truncatedToken)

	// Validate JWT token using JWKS (with smart caching)
	token, err := validateJWTWithJWKS(jwtToken)
	if err != nil {
		fmt.Printf("JWT validation failed for %s operation on namespace %s: %v\n", operation, namespace, err)
		http.Error(w, "Invalid JWT token", http.StatusUnauthorized)
		return
	}

	// Extract and log claims
	subject := token.Subject()
	issuer := token.Issuer()
	audience := token.Audience()
	expiration := token.Expiration()

	fmt.Printf("JWT validated successfully:\n")
	fmt.Printf("  Subject: %s\n", subject)
	fmt.Printf("  Issuer: %s\n", issuer)
	fmt.Printf("  Audience: %v\n", audience)
	fmt.Printf("  Expires: %v\n", expiration)
	fmt.Printf("  Namespace: %s\n", namespace)
	fmt.Printf("  Operation: %s\n", operation)

	// Handle the codec operation
	if r.Method != http.MethodPost {
		fmt.Printf("Invalid method %s for %s operation\n", r.Method, operation)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req struct {
		Payloads []*commonpb.Payload `json:"payloads"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Printf("Failed to decode request body for %s operation: %v\n", operation, err)
		http.Error(w, "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	fmt.Printf("Processing %d payloads for %s operation\n", len(req.Payloads), operation)

	// Apply codec chain
	result := req.Payloads
	for i, codec := range codecChain {
		if isEncode {
			result, err = codec.Encode(result)
		} else {
			result, err = codec.Decode(result)
		}
		if err != nil {
			fmt.Printf("Codec error at position %d for %s operation: %v\n", i, operation, err)
			http.Error(w, fmt.Sprintf("Codec error: %v", err), http.StatusInternalServerError)
			return
		}
	}

	fmt.Printf("Successfully processed %s operation for namespace %s (%d payloads)\n",
		operation, namespace, len(result))

	// Return result
	w.Header().Set("Content-Type", "application/json")
	resp := struct {
		Payloads []*commonpb.Payload `json:"payloads"`
	}{Payloads: result}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		fmt.Printf("Failed to encode response for %s operation: %v\n", operation, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// Simple request logging middleware
func requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		fmt.Printf("Request: %s %s from %s\n", r.Method, r.URL.Path, r.RemoteAddr)
		if namespace := r.Header.Get("X-Namespace"); namespace != "" {
			fmt.Printf("  X-Namespace: %s\n", namespace)
		}

		next.ServeHTTP(w, r)

		fmt.Printf("Request completed in %v\n", time.Since(start))
	})
}

var portFlag int
var webFlag string
var certFlag string
var keyFlag string

func init() {
	flag.IntVar(&portFlag, "port", 8081, "Port to listen on")
	flag.StringVar(&webFlag, "web", "", "Temporal Web URL. Optional: enables CORS which is required for access from Temporal Web")
	flag.StringVar(&certFlag, "cert", "", "Path to TLS certificate file (enables HTTPS)")
	flag.StringVar(&keyFlag, "key", "", "Path to TLS key file (required with -cert)")
}

func main() {
	flag.Parse()

	// No need for explicit cache initialization with hybrid approach
	// Cache will be populated on first JWT validation request

	// Validate TLS flags
	if (certFlag == "") != (keyFlag == "") {
		fmt.Println("Both -cert and -key flags must be provided together")
		os.Exit(1)
	}

	// Configure codecs for supported namespaces
	codecs := map[string][]converter.PayloadCodec{
		"default":                 {temporalconverters.NewPayloadCodec()},
		"tusharb-demo-mtls.sdvdw": {temporalconverters.NewPayloadCodec()},
	}

	handler := newPayloadCodecNamespacesHTTPHandler(codecs)
	handler = requestLoggingMiddleware(handler)

	if webFlag != "" {
		fmt.Printf("CORS enabled for Origin: %s\n", webFlag)
		handler = newCORSHTTPHandler(webFlag, handler)
	}

	srv := &http.Server{
		Addr:    "0.0.0.0:" + strconv.Itoa(portFlag),
		Handler: handler,
	}

	errCh := make(chan error, 1)
	if certFlag != "" && keyFlag != "" {
		fmt.Printf("Starting HTTPS server on port %d\n", portFlag)
		go func() { errCh <- srv.ListenAndServeTLS(certFlag, keyFlag) }()
	} else {
		fmt.Printf("Starting HTTP server on port %d\n", portFlag)
		go func() { errCh <- srv.ListenAndServe() }()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	select {
	case <-sigCh:
		fmt.Println("Shutting down server...")
		_ = srv.Close()
	case err := <-errCh:
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}
