package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type RPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
}

type RPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	Result  any       `json:"result"`
	Error   *RPCError `json:"error"`
	ID      int       `json:"id"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var (
	ErrNoRPCsFound = fmt.Errorf("all known rpc urls are failing. Try searching for it manually or increase the timeout")
)

func FindAllWorkingRPCs(rpcURLs []string, expectedChainID uint64, timeout time.Duration) ([]string, error) {
	workingRPCs := findWorkingRPCsConcurrently(rpcURLs, expectedChainID, timeout)
	if len(workingRPCs) == 0 {
		return nil, ErrNoRPCsFound
	}
	return workingRPCs, nil
}

func FindRandomWorkingRPC(rpcURLs []string, expectedChainID uint64, timeout time.Duration) (string, error) {
	workingRPCs := findWorkingRPCsConcurrently(rpcURLs, expectedChainID, timeout)
	if len(workingRPCs) == 0 {
		return "", ErrNoRPCsFound
	}

	// Return a random working RPC
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomIndex := r.Intn(len(workingRPCs))
	return workingRPCs[randomIndex], nil
}

func findWorkingRPCsConcurrently(rpcURLs []string, expectedChainID uint64, timeout time.Duration) []string {
	var workingRPCs []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Channel to signal when timeout is reached
	timeoutCh := time.After(timeout)
	resultCh := make(chan string, len(rpcURLs))

	// Test all RPCs concurrently
	for _, rpcURL := range rpcURLs {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			if isRPCWorkingWithTimeout(url, expectedChainID, timeout) {
				select {
				case resultCh <- url:
				case <-timeoutCh:
					// Timeout reached, don't add to results
				}
			}
		}(rpcURL)
	}

	// Wait for all tests to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Collect results until timeout or all tests complete
	for {
		select {
		case url := <-resultCh:
			mu.Lock()
			workingRPCs = append(workingRPCs, url)
			mu.Unlock()
		case <-timeoutCh:
			return workingRPCs
		case <-done:
			// Drain any remaining results
			for {
				select {
				case url := <-resultCh:
					mu.Lock()
					workingRPCs = append(workingRPCs, url)
					mu.Unlock()
				default:
					return workingRPCs
				}
			}
		}
	}
}

func isRPCWorkingWithTimeout(rpcURL string, expectedChainID uint64, timeout time.Duration) bool {
	if isWebSocketURL(rpcURL) {
		return isWebSocketRPCWorking(rpcURL, expectedChainID, timeout)
	}
	return isHTTPRPCWorking(rpcURL, expectedChainID, timeout)
}

func isWebSocketURL(rpcURL string) bool {
	return strings.HasPrefix(rpcURL, "wss://")
}

func isHTTPRPCWorking(rpcURL string, expectedChainID uint64, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	request := RPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_chainId",
		Params:  []any{},
		ID:      1,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return false
	}

	req, err := http.NewRequestWithContext(ctx, "POST", rpcURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return false
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false
	}

	var rpcResp RPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return false
	}

	if rpcResp.Error != nil {
		return false
	}

	chainIDHex, ok := rpcResp.Result.(string)
	if !ok {
		return false
	}

	chainID, err := strconv.ParseUint(chainIDHex, 0, 64)
	if err != nil {
		return false
	}

	return chainID == expectedChainID
}

func isWebSocketRPCWorking(rpcURL string, expectedChainID uint64, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Parse URL for websocket connection
	u, err := url.Parse(rpcURL)
	if err != nil {
		return false
	}

	// Create websocket dialer with timeout
	dialer := websocket.Dialer{
		HandshakeTimeout: timeout,
	}

	// Connect to websocket
	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return false
	}
	defer conn.Close()

	// Set read/write deadlines
	deadline := time.Now().Add(timeout)
	conn.SetReadDeadline(deadline)
	conn.SetWriteDeadline(deadline)

	// Prepare RPC request
	request := RPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_chainId",
		Params:  []any{},
		ID:      1,
	}

	// Send JSON-RPC request
	if err := conn.WriteJSON(request); err != nil {
		return false
	}

	// Read response
	var rpcResp RPCResponse
	if err := conn.ReadJSON(&rpcResp); err != nil {
		return false
	}

	if rpcResp.Error != nil {
		return false
	}

	chainIDHex, ok := rpcResp.Result.(string)
	if !ok {
		return false
	}

	chainID, err := strconv.ParseUint(chainIDHex, 0, 64)
	if err != nil {
		return false
	}

	return chainID == expectedChainID
}
