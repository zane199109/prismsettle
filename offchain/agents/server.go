package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zane/web3-offchain/pkg/logger"
)

// Server wraps an Agent in an HTTP server exposing /invoke and
// /.well-known/agent.json. It is shared by all 4 agents.
type Server struct {
	agent Agent
	llm   *LLMClient
}

// NewServer builds a Server for the given agent.
func NewServer(agent Agent, llm *LLMClient) *Server {
	return &Server{agent: agent, llm: llm}
}

// invokeRequest is the body of POST /invoke (FR-AP01).
type invokeRequest struct {
	Input  string `json:"input"`
	Caller string `json:"caller"`
}

// invokeResponse is the body of POST /invoke (FR-AP02).
type invokeResponse struct {
	Output    string `json:"output"`
	ProofHash string `json:"proof_hash"`
	AgentName string `json:"agent_name"`
	TookMs    int64  `json:"took_ms"`
}

// Serve starts the HTTP server on the given port and blocks until the context
// is canceled or the process receives SIGINT/SIGTERM. SIGHUP triggers a
// config reload (hot-reload of API keys / prompts).
func (s *Server) Serve(ctx context.Context, port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/invoke", s.handleInvoke)
	mux.HandleFunc("/.well-known/agent.json", s.handleMetadata)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	addr := fmt.Sprintf(":%d", port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	// SIGHUP: hot-reload config (API key rotation / prompt tuning).
	sighupCh := make(chan os.Signal, 1)
	signal.Notify(sighupCh, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sighupCh:
				if _, err := ReloadConfig(); err != nil {
					logger.Errorf("config reload failed", logger.Error(err))
					continue
				}
				logger.Info("agents config reloaded via SIGHUP",
					logger.String("agent", s.agent.Name()))
			}
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("agent server starting",
			logger.String("agent", s.agent.Name()),
			logger.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) handleInvoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req invokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json: %v", err)
		return
	}
	if req.Input == "" {
		writeErr(w, http.StatusBadRequest, "input is required")
		return
	}
	if req.Caller == "" {
		req.Caller = "0x0000000000000000000000000000000000000000"
	}

	start := time.Now()
	output, proofHash, err := s.agent.Invoke(req.Input, req.Caller)
	tookMs := time.Since(start).Milliseconds()
	if err != nil {
		logger.Errorf("agent invoke failed",
			logger.String("agent", s.agent.Name()),
			logger.Error(err))
		writeErr(w, http.StatusInternalServerError, "agent failed: %v", err)
		return
	}

	resp := invokeResponse{
		Output:    output,
		ProofHash: proofHash,
		AgentName: s.agent.Name(),
		TookMs:    tookMs,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleMetadata(w http.ResponseWriter, r *http.Request) {
	md := s.agent.Metadata()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(md)
}

func writeErr(w http.ResponseWriter, code int, format string, args ...interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": fmt.Sprintf(format, args...),
	})
}
