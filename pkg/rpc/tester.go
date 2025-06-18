package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
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

func FindWorkingRPC(rpcURLs []string, expectedChainID uint64) (string, error) {
	for _, rpcURL := range rpcURLs {
		if isRPCWorking(rpcURL, expectedChainID) {
			return rpcURL, nil
		}
	}
	return "", fmt.Errorf("All known rpc urls are failing. Try searching for it manually.")
}

func FindAllWorkingRPCs(rpcURLs []string, expectedChainID uint64, timeout time.Duration) []string {
	return findWorkingRPCsConcurrently(rpcURLs, expectedChainID, timeout)
}

func FindRandomWorkingRPC(rpcURLs []string, expectedChainID uint64, timeout time.Duration) (string, error) {
	workingRPCs := findWorkingRPCsConcurrently(rpcURLs, expectedChainID, timeout)
	if len(workingRPCs) == 0 {
		return "", fmt.Errorf("All known rpc urls are failing. Try searching for it manually.")
	}
	
	// Return a random working RPC
	rand.Seed(time.Now().UnixNano())
	randomIndex := rand.Intn(len(workingRPCs))
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

func isRPCWorking(rpcURL string, expectedChainID uint64) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
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

func isRPCWorkingWithTimeout(rpcURL string, expectedChainID uint64, timeout time.Duration) bool {
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