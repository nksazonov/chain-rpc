package chain

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RPC struct {
	URL      string `json:"url"`
	Tracking string `json:"tracking"`
}

type NativeCurrency struct {
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

type Explorer struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Standard string `json:"standard"`
}

type ChainData struct {
	Name           string         `json:"name"`
	Chain          string         `json:"chain"`
	RPCs           []RPC          `json:"rpc"`
	NativeCurrency NativeCurrency `json:"nativeCurrency"`
	ShortName      string         `json:"shortName"`
	ChainID        uint64         `json:"chainId"`
	Explorers      []Explorer     `json:"explorers"`
	ChainSlug      string         `json:"chainSlug"`
}

type NameToIdMap = map[string]uint64

type CacheData struct {
	ByID   map[uint64]*ChainData `json:"byId"`
	ByName NameToIdMap           `json:"byName"`
}

var (
	cacheMux     sync.RWMutex
	cacheFile    string
	isVerbose    bool
	forceRebuild bool
)

const (
	CHAINS_DATA_URL = "https://chainlist.org/rpcs.json"
	CACHE_TTL       = 30 * 24 * time.Hour // 1 month
)

var (
	ErrChainNotFound = fmt.Errorf("specified chain does not exist or is not known at `chainlist.org`")
)

func SetVerbose(verbose bool) {
	isVerbose = verbose
}

func SetForceRebuild(force bool) {
	forceRebuild = force
}

func normalizeChainName(name string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), " ", "-")
}

func verbosePrintf(format string, args ...any) {
	if isVerbose {
		fmt.Printf(format, args...)
	}
}

func init() {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		userCacheDir = os.TempDir()
	}
	cacheDir := filepath.Join(userCacheDir, "chain-rpc")
	os.MkdirAll(cacheDir, 0755)
	cacheFile = filepath.Join(cacheDir, "cache.json")
}

func FetchChainData(chainId uint64) (*ChainData, error) {
	if err := ensureCacheExists(); err != nil {
		return nil, err
	}

	return loadChainByID(chainId)
}

func FetchChainDataByName(name string) (*ChainData, error) {
	if err := ensureCacheExists(); err != nil {
		return nil, err
	}

	return loadChainByName(name)
}

func ensureCacheExists() error {
	cacheMux.Lock()
	defer cacheMux.Unlock()

	// Check if cache file exists and is not expired (unless force rebuild is requested)
	cacheExists := false
	if !forceRebuild {
		if stat, err := os.Stat(cacheFile); err == nil {
			// Check if cache is not expired
			if time.Since(stat.ModTime()) < CACHE_TTL {
				cacheExists = true
			}
		}
	}

	if cacheExists {
		return nil
	}

	// Cache doesn't exist, is invalid, or expired - try to build it
	if err := buildCache(); err != nil {
		// If we failed to build cache but have an old cache, use it
		if _, readErr := os.Stat(cacheFile); readErr == nil {
			verbosePrintf("Warning: Failed to update cache (%v), using existing cache\n", err)
			return nil
		}
		// No existing cache and failed to build new one
		return err
	}

	return nil
}

func buildCache() error {
	verbosePrintf("Fetching and building chain data cache...\n")

	// Fetch all chains data
	resp, err := http.Get(CHAINS_DATA_URL)
	if err != nil {
		return fmt.Errorf("failed to fetch chains data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to fetch chains data: HTTP %d", resp.StatusCode)
	}

	var chains []ChainData
	if err := json.NewDecoder(resp.Body).Decode(&chains); err != nil {
		return fmt.Errorf("failed to parse chains data: %v", err)
	}

	// Process chains concurrently
	cacheData := &CacheData{
		ByID:   make(map[uint64]*ChainData),
		ByName: make(NameToIdMap),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := range chains {
		wg.Add(1)
		go func(chain *ChainData) {
			defer wg.Done()

			mu.Lock()
			cacheData.ByID[chain.ChainID] = chain

			// Add multiple name mappings for better lookup
			if chain.Name != "" {
				cacheData.ByName[normalizeChainName(chain.Name)] = chain.ChainID
			}
			if chain.ShortName != "" {
				cacheData.ByName[normalizeChainName(chain.ShortName)] = chain.ChainID
			}
			if chain.ChainSlug != "" {
				cacheData.ByName[normalizeChainName(chain.ChainSlug)] = chain.ChainID
			}
			mu.Unlock()
		}(&chains[i])
	}

	wg.Wait()

	// Save to cache file
	data, err := json.Marshal(cacheData)
	if err != nil {
		return fmt.Errorf("failed to serialize cache: %v", err)
	}

	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache: %v", err)
	}

	verbosePrintf("Cache built successfully with %d chains\n", len(cacheData.ByID))
	return nil
}

func loadChainByID(chainId uint64) (*ChainData, error) {
	file, err := os.Open(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open cache file: %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.UseNumber()

	// Read opening brace
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("failed to read cache file: %v", err)
	}

	// Read through the cache structure
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("failed to read cache file: %v", err)
		}

		if str, ok := token.(string); ok && str == "byId" {
			// Found byId section, now look for our chain ID
			return findChainInByID(decoder, chainId)
		} else {
			// Skip this field
			if err := skipValue(decoder); err != nil {
				return nil, err
			}
		}
	}

	return nil, ErrChainNotFound
}

func loadChainByName(name string) (*ChainData, error) {
	normalizedName := normalizeChainName(name)

	// First, find the chain ID from byName mapping
	chainId, err := findChainIDByName(normalizedName)
	if err != nil {
		return nil, err
	}

	// Then load the chain data by ID
	return loadChainByID(chainId)
}

func loadNameMapping() (NameToIdMap, error) {
	file, err := os.Open(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open cache file: %v", err)
	}
	defer file.Close()

	var cacheData CacheData
	if err := json.NewDecoder(file).Decode(&cacheData); err != nil {
		return nil, fmt.Errorf("failed to decode cache file: %v", err)
	}

	return cacheData.ByName, nil
}

func findChainIDByName(normalizedName string) (uint64, error) {
	// Load the entire name mapping into memory
	nameMapping, err := loadNameMapping()
	if err != nil {
		return 0, err
	}

	// Look up the chain ID
	chainID, exists := nameMapping[normalizedName]
	if !exists {
		// look for ethereum mainnet or ethereum-<name> variations
		if chainId, err := findChainIdByPartialMatch(nameMapping, "ethereum-"+normalizedName); err == nil {
			return chainId, nil
		}
		// look for mainnet variations
		if chainId, err := findChainIdByPartialMatch(nameMapping, normalizedName+"-mainnet"); err == nil {
			return chainId, nil
		}

		// look for partial matches
		return findChainIdByPartialMatch(nameMapping, normalizedName)
	}

	return chainID, nil
}

func findChainIdByPartialMatch(nameMapping NameToIdMap, name string) (uint64, error) {
	matchingKeys := make([]string, 0)
	for key := range nameMapping {
		if strings.Contains(key, name) {
			matchingKeys = append(matchingKeys, key)
		}
	}

	if len(matchingKeys) == 1 {
		return nameMapping[matchingKeys[0]], nil
	} else if len(matchingKeys) > 1 {
		errMsg := fmt.Sprintf("found multiple chains matching '%s':\n", name)
		for _, key := range matchingKeys {
			errMsg += fmt.Sprintf("- %s\n", key)
		}
		return 0, fmt.Errorf("%s \nPlease specify a more precise name", errMsg)
	}

	return 0, fmt.Errorf("chain not found for name '%s'", name)
}

func findChainInByID(decoder *json.Decoder, targetChainId uint64) (*ChainData, error) {
	// Read opening brace of byId object
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("failed to read byId object: %v", err)
	}

	// Read through byId entries
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("failed to read byId entry: %v", err)
		}

		if str, ok := token.(string); ok {
			// Parse the chain ID from the key
			chainId, err := strconv.ParseUint(str, 10, 64)
			if err != nil {
				// Skip invalid entries
				if err := skipValue(decoder); err != nil {
					return nil, err
				}
				continue
			}

			if chainId == targetChainId {
				// Found our chain, decode it
				var chainData ChainData
				if err := decoder.Decode(&chainData); err != nil {
					return nil, fmt.Errorf("failed to decode chain data: %v", err)
				}
				return &chainData, nil
			} else {
				// Skip this chain data
				if err := skipValue(decoder); err != nil {
					return nil, err
				}
			}
		} else {
			return nil, fmt.Errorf("unexpected token in byId: %v", token)
		}
	}

	return nil, ErrChainNotFound
}

func skipValue(decoder *json.Decoder) error {
	// Read one JSON value and discard it
	var discard interface{}
	return decoder.Decode(&discard)
}

func CleanCache() error {
	cacheMux.Lock()
	defer cacheMux.Unlock()

	if err := os.Remove(cacheFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache file: %v", err)
	}

	verbosePrintf("Cache cleaned successfully\n")
	return nil
}

func BuildCache() error {
	cacheMux.Lock()
	defer cacheMux.Unlock()

	return buildCache()
}
