package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"chain-rpc/pkg/chain"
	"chain-rpc/pkg/rpc"

	"github.com/spf13/cobra"
)

const (
	version = "0.1.1"

	// ANSI color codes
	colorRed   = "\033[31m"
	colorReset = "\033[0m"
)

var (
	noTest    bool
	verbose   bool
	force     bool
	timeout   time.Duration
	wsOnly    bool
	httpsOnly bool
)

var rootCmd = &cobra.Command{
	Use:   "chain-rpc <chainId|chainName>",
	Short: "Find first working RPC endpoint for a blockchain network",
	Long:  "Fetches chain data from `chainlist.org` and tests RPC endpoints to find the first working one. Accepts either chain ID (number) or chain name (string)",
	Args:  exactArgsWithParameterError(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chain.SetVerbose(verbose)
		chain.SetForceRebuild(force)

		chainData, err := getChainData(args[0])
		if err != nil {
			return err
		}

		rpcUrls := extractRPCUrls(chainData.RPCs, wsOnly, httpsOnly)
		if len(rpcUrls) == 0 {
			return fmt.Errorf("no known rpc urls for this chain at `chainlist.org`")
		}

		if noTest {
			fmt.Println(rpcUrls[0])
			return nil
		}

		workingRPC, err := rpc.FindRandomWorkingRPC(rpcUrls, chainData.ChainID, timeout)
		if err != nil {
			return err
		}

		fmt.Println(workingRPC)
		return nil
	},
}

var allCmd = &cobra.Command{
	Use:   "all <chainId|chainName>",
	Short: "Find all working RPC endpoints for a blockchain network",
	Long:  "Fetches chain data from ethereum-lists/chains and tests all RPC endpoints to find working ones. Accepts either chain ID (number) or chain name (string)",
	Args:  exactArgsWithParameterError(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chain.SetVerbose(verbose)
		chain.SetForceRebuild(force)

		chainData, err := getChainData(args[0])
		if err != nil {
			return err
		}

		rpcUrls := extractRPCUrls(chainData.RPCs, wsOnly, httpsOnly)
		if len(rpcUrls) == 0 {
			return fmt.Errorf("no known rpc urls for this chain at `chainlist.org`")
		}

		if noTest {
			for _, rpcURL := range rpcUrls {
				fmt.Println(rpcURL)
			}
			return nil
		}

		workingRPCs, err := rpc.FindAllWorkingRPCs(rpcUrls, chainData.ChainID, timeout)
		if err != nil {
			return err
		}

		// Shuffle the results for better load distribution
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		r.Shuffle(len(workingRPCs), func(i, j int) {
			workingRPCs[i], workingRPCs[j] = workingRPCs[j], workingRPCs[i]
		})

		for _, rpcURL := range workingRPCs {
			fmt.Println(rpcURL)
		}
		return nil
	},
}

func getChainData(identifier string) (*chain.ChainData, error) {
	// Try to parse as chain ID first
	if chainId, err := strconv.ParseUint(identifier, 10, 64); err == nil {
		return chain.FetchChainData(chainId)
	}

	// If not a number, treat as chain name
	return chain.FetchChainDataByName(identifier)
}

func extractRPCUrls(rpcs []chain.RPC, wsOnly, httpsOnly bool) []string {
	urls := make([]string, 0, len(rpcs))
	for _, rpc := range rpcs {
		if rpc.URL != "" {
			// Apply filtering based on flags
			if wsOnly && !isWebSocketURL(rpc.URL) {
				continue
			}
			if httpsOnly && !isHTTPSURL(rpc.URL) {
				continue
			}
			urls = append(urls, rpc.URL)
		}
	}
	return urls
}

func isWebSocketURL(url string) bool {
	return strings.HasPrefix(url, "ws://") || strings.HasPrefix(url, "wss://")
}

func isHTTPSURL(url string) bool {
	return strings.HasPrefix(url, "https://")
}

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage chain data cache",
	Long:  "Commands to manage the local chain data cache",
}

var cacheCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove the cache file",
	Long:  "Removes the local cache file, forcing a fresh download on next use",
	RunE: func(cmd *cobra.Command, args []string) error {
		return chain.CleanCache()
	},
}

var cacheBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build/update the cache file",
	Long:  "Downloads fresh chain data and rebuilds the cache file",
	RunE: func(cmd *cobra.Command, args []string) error {
		return chain.BuildCache()
	},
}

var idCmd = &cobra.Command{
	Use:   "id <chainName>",
	Short: "Get chain ID from chain name",
	Long:  "Returns the chain ID for the given chain name",
	Args:  exactArgsWithParameterError(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chain.SetVerbose(verbose)
		chain.SetForceRebuild(force)

		chainData, err := chain.FetchChainDataByName(args[0])
		if err != nil {
			return err
		}

		fmt.Println(chainData.ChainID)
		return nil
	},
}

var nameCmd = &cobra.Command{
	Use:   "name <chainId>",
	Short: "Get chain name from chain ID",
	Long:  "Returns the chain name for the given chain ID",
	Args:  exactArgsWithParameterError(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chain.SetVerbose(verbose)
		chain.SetForceRebuild(force)

		chainId, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return NewParameterErrorWithCmd("chainId must be a valid number", cmd)
		}

		chainData, err := chain.FetchChainData(chainId)
		if err != nil {
			return err
		}

		fmt.Println(chainData.Name)
		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Long:  "Print the version number of chain-rpc",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}

func init() {
	rootCmd.Flags().BoolVar(&noTest, "no-test", false, "return RPC URLs without testing them")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.Flags().BoolVarP(&force, "force", "f", false, "force rebuild cache")
	rootCmd.Flags().DurationVarP(&timeout, "timeout", "t", 200*time.Millisecond, "timeout for RPC testing")
	rootCmd.Flags().BoolVar(&wsOnly, "wss", false, "return only WebSocket RPC URLs")
	rootCmd.Flags().BoolVar(&httpsOnly, "https", false, "return only HTTPS RPC URLs")

	allCmd.Flags().BoolVar(&noTest, "no-test", false, "return all RPC URLs without testing them")
	allCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	allCmd.Flags().BoolVarP(&force, "force", "f", false, "force rebuild cache")
	allCmd.Flags().DurationVarP(&timeout, "timeout", "t", 200*time.Millisecond, "timeout for RPC testing")
	allCmd.Flags().BoolVar(&wsOnly, "wss", false, "return only WebSocket RPC URLs")
	allCmd.Flags().BoolVar(&httpsOnly, "https", false, "return only HTTPS RPC URLs")

	cacheCmd.AddCommand(cacheCleanCmd)
	cacheCmd.AddCommand(cacheBuildCmd)

	idCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	idCmd.Flags().BoolVarP(&force, "force", "f", false, "force rebuild cache")

	nameCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	nameCmd.Flags().BoolVarP(&force, "force", "f", false, "force rebuild cache")

	// Set SilenceUsage and SilenceErrors for all commands to prevent automatic output on errors
	commands := []*cobra.Command{rootCmd, allCmd, idCmd, nameCmd, cacheCmd, cacheCleanCmd, cacheBuildCmd, versionCmd}
	for _, cmd := range commands {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
	}

	// Handle flag errors by converting to ParameterError for all commands
	flagErrorFunc := func(cmd *cobra.Command, err error) error {
		return NewParameterErrorWithCmd(err.Error(), cmd)
	}

	for _, cmd := range commands {
		cmd.SetFlagErrorFunc(flagErrorFunc)
	}

	rootCmd.AddCommand(allCmd)
	rootCmd.AddCommand(cacheCmd)
	rootCmd.AddCommand(idCmd)
	rootCmd.AddCommand(nameCmd)
	rootCmd.AddCommand(versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, formatError(err))
		if paramErr, ok := err.(*ParameterError); ok {
			fmt.Fprintln(os.Stderr, "")
			if paramErr.cmd != nil {
				paramErr.cmd.Help()
			} else {
				rootCmd.Help()
			}
		}
		os.Exit(1)
	}
}
