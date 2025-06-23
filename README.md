# chain-rpc

A fast and reliable CLI tool for finding working RPC endpoints for blockchain networks. It fetches chain data from [chainlist.org](https://chainlist.org) and tests RPC endpoints to identify working ones.

## Features

- **Fast RPC Discovery**: Concurrently tests multiple RPC endpoints to find working ones quickly
- **Smart Chain Lookup**: Intelligent chain name search with fallback strategies
- **Protocol Support**: Full support for both HTTP/HTTPS and WebSocket RPC endpoints
- **Protocol Filtering**: Filter results by protocol type (--https, --wss)
- **Smart Caching**: Local cache with 30-day TTL for faster subsequent lookups
- **Multiple Output Modes**: Get first working RPC, all working RPCs, or untested URLs
- **Chain Info**: Retrieve chain names and IDs for reference
- **Timeout Control**: Configurable timeout for RPC testing (default: 200ms)

## Usage

### Basic Commands

#### Find first working RPC endpoint

```bash
# By chain ID
chain-rpc 1                    # Ethereum Mainnet
chain-rpc 137                  # Polygon

# By chain name
chain-rpc ethereum             # Ethereum Mainnet
chain-rpc polygon              # Polygon
```

#### Find all working RPC endpoints

```bash
chain-rpc all 1                # All working Ethereum RPCs
chain-rpc all polygon          # All working Polygon RPCs
```

#### Get chain information

```bash
chain-rpc id ethereum          # Returns: 1
chain-rpc name 1               # Returns: Ethereum Mainnet
```

### Options

#### Global Flags

- `--no-test`: Return RPC URLs without testing them
- `--https`: Return only HTTPS RPC URLs
- `--wss`: Return only WebSocket (WSS) RPC URLs
- `-v, --verbose`: Enable verbose output
- `-f, --force`: Force rebuild cache
- `-t, --timeout duration`: Timeout for RPC testing (default: 200ms)

#### Examples with flags

```bash
# Get untested RPC URLs (fastest)
chain-rpc 1 --no-test

# Get only HTTPS endpoints
chain-rpc 1 --https

# Get only WebSocket endpoints
chain-rpc ethereum --wss

# Find working RPC with longer timeout
chain-rpc 1 --timeout 5s

# Verbose output with cache rebuild
chain-rpc polygon --verbose --force

# Get all HTTPS RPCs without testing
chain-rpc all 1 --https --no-test

# Get all WebSocket RPCs for Polygon
chain-rpc all polygon --wss
```

### Cache Management

#### Build/update cache

```bash
chain-rpc cache build
```

#### Clean cache

```bash
chain-rpc cache clean
```

The cache is automatically managed and stored in your system's cache directory (`~/Library/Caches/chain-rpc/` on Linux/macOS).

## How It Works

1. **Data Source**: Fetches blockchain network data from [chainlist.org/rpcs.json](https://chainlist.org/rpcs.json)
2. **Smart Chain Search**: Multi-tier lookup strategy:
   - Direct match (e.g., `linea-mainnet`)
   - Ethereum chains (e.g., `ethereum-sepolia`)  
   - Mainnet chains (e.g., `base-mainnet`)
   - Partial match (e.g., `on-xdai` in `arbitrum-on-xdai`)
3. **Caching**: Stores data locally for 30 days to avoid repeated API calls
4. **Protocol Support**: Tests both HTTP/HTTPS and WebSocket endpoints
5. **RPC Testing**: Tests endpoints using `eth_chainId` JSON-RPC call
6. **Concurrent Testing**: Tests multiple endpoints simultaneously for speed
7. **Chain Validation**: Ensures returned chain ID matches the expected one

## Architecture

The tool is structured into three main packages:

- **`main`**: CLI interface using Cobra framework
- **`pkg/chain`**: Chain data fetching, caching, and lookup functionality
- **`pkg/rpc`**: RPC endpoint testing and validation

### Key Components

#### Chain Data Management (`pkg/chain/fetcher.go`)

- Fetches data from chainlist.org
- Implements efficient caching with TTL
- Supports lookup by chain ID, name, short name, or slug
- Thread-safe operations with mutex protection

#### RPC Testing (`pkg/rpc/tester.go`)

- Concurrent testing of multiple endpoints
- Support for both HTTP/HTTPS and WebSocket protocols
- Configurable timeouts
- Chain ID validation using `eth_chainId` method
- Load balancing through result shuffling

## Performance

- **Cache Hit**: Near-instant response from local cache
- **Cache Miss**: ~1-3 seconds to fetch and build cache
- **RPC Testing**: Concurrent testing with 200ms default timeout
- **Memory Efficient**: Streams large JSON files without loading everything into memory

## Supported Networks

Supports all blockchain networks listed on [chainlist.org](https://chainlist.org), including:

- Ethereum Mainnet (1)
- Polygon (137)
- Binance Smart Chain (56)
- Avalanche (43114)
- Arbitrum (42161)
- Optimism (10)
- And 2000+ other networks

## Error Handling

The tool provides clear error messages for common issues:

- `specified chain does not exist or is not known at chainlist.org` - Invalid chain ID/name
- `no known rpc urls for this chain at chainlist.org` - Chain has no RPC endpoints
- `all known rpc urls are failing` - All endpoints are down or unreachable
- Network/timeout errors are handled gracefully with fallback to cached data

## Examples

```bash
# Quick RPC for Ethereum
$ chain-rpc 1
https://rpc.flashbots.net/fast

# Get WebSocket endpoint for Ethereum
$ chain-rpc ethereum --wss
wss://ethereum-rpc.publicnode.com

# All working Polygon HTTPS RPCs
$ chain-rpc all 137 --https
https://polygon-rpc.com
https://rpc-mainnet.matic.network
https://matic-mainnet.chainstacklabs.com

# Chain lookup with smart search
$ chain-rpc id "binance smart chain"
56

# Partial name matching
$ chain-rpc id "on-xdai"  # matches arbitrum-on-xdai
200

# Get untested URLs (fastest)
$ chain-rpc ethereum --no-test
https://eth.llamarpc.com

# Get only WebSocket URLs without testing
$ chain-rpc polygon --wss --no-test
wss://polygon-bor-rpc.publicnode.com

# Force cache refresh
$ chain-rpc polygon --force --verbose
Fetching and building chain data cache...
Cache built successfully with 1247 chains
https://polygon-mainnet.g.alchemy.com/v2/demo
```

## Requirements

- Go 1.21.0 or later
- Internet connection for initial cache building
- ~10MB disk space for cache storage

## Dependencies

- [github.com/spf13/cobra](https://github.com/spf13/cobra) - CLI framework
- [github.com/gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket support
