package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/scaleoutsean/solidfire-go/sdk"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// UserContext represents the identity extracted from an OIDC token.
// In OIDC, you typically get a 'sub' (subject) for ID and 'groups' for RBAC.
type UserContext struct {
	ID    string   // e.g. "auth0|12345" or "joe@example.com"
	Roles []string // e.g. ["DATAFABRICLAN\\SFTENANT004"]
}

func initLogging(conf LoggingConfig) {
	// 1. Set Format
	if conf.Format == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	}

	// 2. Set Level
	level, err := log.ParseLevel(conf.Level)
	if err != nil {
		level = log.InfoLevel
	}
	log.SetLevel(level)

	// 3. Set Output (Stdout, File, or MultiWriter)
	var outputs []io.Writer
	outputs = append(outputs, os.Stdout)

	if conf.Output == "file" && conf.FilePath != "" {
		f, err := os.OpenFile(conf.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			outputs = append(outputs, f)
		}
	}

	// 4. Advanced: TLS Syslog Forwarding
	if conf.SyslogAddr != "" {
		// For TLS Syslog, we use a encrypted TCP connection as a writer.
		// In production, you would use a proper syslog hook, but this demonstrates
		// the raw capability of forwarding logs over TLS.
		tlsConf := &tls.Config{InsecureSkipVerify: false}
		conn, err := tls.Dial("tcp", conf.SyslogAddr, tlsConf)
		if err == nil {
			outputs = append(outputs, conn)
			log.Infof("Telemetry: Forwarding audit logs to TLS Syslog at %s", conf.SyslogAddr)
		} else {
			log.Errorf("Failed to connect to TLS Syslog: %v", err)
		}
	}

	log.SetOutput(io.MultiWriter(outputs...))
}

func main() {
	// 1. Parse command line flags or environment variables
	configPath := flag.String("config", os.Getenv("PROXY_CONFIG"), "Path to YAML configuration file")
	flag.Parse()

	if *configPath == "" {
		log.Fatal("Configuration path must be provided via -config flag or PROXY_CONFIG env var")
	}

	// 2. Load configuration from YAML
	yamlFile, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	var conf Config
	if err := yaml.Unmarshal(yamlFile, &conf); err != nil {
		log.Fatalf("Error parsing config file: %v", err)
	}

	initLogging(conf.Logging)
	log.Infof("Starting Secure SolidFire Proxy with config from %s", *configPath)

	targetURL, _ := url.Parse(conf.Clusters["PROD"].Endpoint)

	// 2. Setup the Reverse Proxy
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Standard Director: Injects Backend Auth and handles routing
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Embed backend credentials into the outgoing request
		req.SetBasicAuth(conf.Clusters["PROD"].Username, conf.Clusters["PROD"].Password)
		req.Host = targetURL.Host
	}

	// 3. The Handler: RBAC + Payload Inspection
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock Auth: In reality, decode your OIDC JWT claims here
		user := UserContext{
			ID:    "joe@example.com",
			Roles: []string{"DATAFABRICLAN\\SFTENANT004"},
		}

		// Read the JSON-RPC body to determine the method being called
		bodyBytes, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Reset body so proxy can read it

		var jsonRPC sdk.BaseRequest
		if err := json.Unmarshal(bodyBytes, &jsonRPC); err != nil {
			http.Error(w, "Invalid JSON-RPC", http.StatusBadRequest)
			return
		}

		log.Printf("[RBAC] User %s calling method %s", user.ID, jsonRPC.Method)

		// --- RBAC Logic Example: Enforce QoS Policy Only ---
		if jsonRPC.Method == "CreateVolume" {
			var cvReq sdk.CreateVolumeRequest
			paramsBits, _ := json.Marshal(jsonRPC.Parameters)
			json.Unmarshal(paramsBits, &cvReq)

			// Logic: If user is not Global Admin, they MUST use a QoSPolicyID
			// and are FORBIDDEN from specifying manual QoS values.
			isGlobalAdmin := false // check user.Roles against conf.GlobalAdminRoles
			if !isGlobalAdmin {
				if cvReq.AccountID != 4 { // Hardcoded tenant check for Joe
					http.Error(w, "RBAC: You are not authorized to create volumes for this account", http.StatusForbidden)
					return
				}
				if cvReq.QosPolicyID == 0 {
					http.Error(w, "RBAC: Provisioning requires a pre-defined QoS Policy ID", http.StatusForbidden)
					return
				}
				if cvReq.Qos != nil {
					http.Error(w, "RBAC: Custom QoS parameters are forbidden for your role", http.StatusForbidden)
					return
				}
			}
		}

		// Proceed to proxy if all checks pass
		proxy.ServeHTTP(w, r)
	})

	// 4. ModifyResponse: Intercept and filter the results
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode != http.StatusOK {
			return nil
		}

		// Read original body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		var sfResp sdk.BaseResponse
		if err := json.Unmarshal(body, &sfResp); err != nil {
			resp.Body = io.NopCloser(bytes.NewReader(body)) // restore body
			return nil
		}

		// Check if the result contains a list of accounts
		resultMap, ok := sfResp.Result.(map[string]interface{})
		if !ok {
			resp.Body = io.NopCloser(bytes.NewReader(body))
			return nil
		}

		if accsRaw, hasAccounts := resultMap["accounts"]; hasAccounts {
			log.Println("[FILTER] Filtering and Redacting ListAccounts response...")

			accsBits, _ := json.Marshal(accsRaw)
			var accounts []sdk.Account
			json.Unmarshal(accsBits, &accounts)

			var filtered []sdk.Account
			allowedTenants := []int64{4} // Match user's allowed list

			for _, acc := range accounts {
				for _, allowed := range allowedTenants {
					if acc.AccountID == allowed {
						acc.Redact() // Using our new Security Helper!
						filtered = append(filtered, acc)
					}
				}
			}

			// Update the result map and re-marshal the entire response
			resultMap["accounts"] = filtered
			sfResp.Result = resultMap
			newBody, _ := json.Marshal(sfResp)

			resp.Body = io.NopCloser(bytes.NewReader(newBody))
			resp.ContentLength = int64(len(newBody))
			resp.Header.Set("Content-Length", fmt.Sprint(len(newBody)))
		} else {
			resp.Body = io.NopCloser(bytes.NewReader(body))
		}
		return nil
	}

	// 5. Hardened TLS 1.3 + PQC Setup
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
		// Post-Quantum Cryptography (PQC)
		// Go 1.24+ supports Kyber/ML-KEM hybrids
		CurvePreferences: []tls.CurveID{
			tls.X25519,    // Standard
			tls.CurveP256, // Standard
		},
	}

	// Add PQC hybrid (Go version 1.24+)
	tlsConfig.CurvePreferences = append([]tls.CurveID{tls.X25519MLKEM768}, tlsConfig.CurvePreferences...)

	server := &http.Server{
		Addr:      conf.Server.ListenAddr,
		Handler:   handler,
		TLSConfig: tlsConfig,
	}

	log.Printf("Secure SolidFire Proxy listening on %s", conf.Server.ListenAddr)
	log.Println("PQC (Post-Quantum) Hybrid Support: Enabled (ML-KEM/Kyber)")

	// server.ListenAndServeTLS("cert.pem", "key.pem")
	fmt.Printf("Server for %s initialized. Pass certs to ListenAndServeTLS to start.\n", server.Addr)
}
