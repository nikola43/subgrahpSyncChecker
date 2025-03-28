package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

// GraphQL response structures
type GraphQLResponse struct {
	Data struct {
		Meta struct {
			Block struct {
				Number int64 `json:"number"`
			} `json:"block"`
		} `json:"_meta"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// SubgraphInfo holds information about each subgraph
type SubgraphInfo struct {
	Chain             string
	Name              string
	URL               string
	CurrentBlock      int64
	LastBlock         int64
	BlocksBehind      int64
	SyncSpeed         float64 // blocks per minute
	EstimatedTimeLeft time.Duration
	LastCheckedBlocks []int64 // store last few block heights for calculating sync speed
	LastCheckedTimes  []time.Time
	MaxHistoryEntries int
}

// ChainInfo holds information about each blockchain
type ChainInfo struct {
	Name        string
	RpcURL      string
	LatestBlock int64
}

var query = `{"query": "{ _meta { block { number } } }"}`

func main() {
	// Define chains to monitor
	chains := map[string]*ChainInfo{
		"pulsechain": {
			Name:   "PulseChain",
			RpcURL: "https://rpc.pulsechain.com",
		},
		"ethereum": {
			Name:   "Ethereum",
			RpcURL: "https://eth.llamarpc.com",
		},
	}

	// Define subgraphs to monitor
	subgraphs := []*SubgraphInfo{
		{
			Name:              "PulseX PulseChain V2",
			URL:               "https://graph.pulsechain.com/subgraphs/name/pulsechain/pulsex",
			MaxHistoryEntries: 6,
			Chain:             "pulsechain",
		},
	}

	// Check once immediately at startup
	checkSubgraphs(subgraphs, chains)

	// Then check every 10 minutes
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		checkSubgraphs(subgraphs, chains)
	}
}

func checkSubgraphs(subgraphs []*SubgraphInfo, chains map[string]*ChainInfo) {
	// First, update the latest block for each chain
	for chainName, chainInfo := range chains {
		latestBlock, err := getLatestBlockFromChain(chainName, chainInfo.RpcURL)
		if err != nil {
			log.Printf("Error getting latest block from %s: %v", chainName, err)
			continue
		}
		chainInfo.LatestBlock = latestBlock
		log.Printf("Latest block for %s: %d", chainInfo.Name, latestBlock)
	}

	// Group subgraphs by chain for the report
	subgraphsByChain := make(map[string][]*SubgraphInfo)
	for _, sg := range subgraphs {
		subgraphsByChain[sg.Chain] = append(subgraphsByChain[sg.Chain], sg)
	}

	// Check each subgraph and print reports by chain
	for chainName, chainSubgraphs := range subgraphsByChain {
		chainInfo, ok := chains[chainName]
		if !ok {
			log.Printf("No chain info found for: %s", chainName)
			continue
		}

		latestBlock := chainInfo.LatestBlock
		if latestBlock == 0 {
			log.Printf("Skipping %s subgraphs as latest block is 0", chainInfo.Name)
			continue
		}

		fmt.Printf("\n--- %s Subgraph Sync Status (Latest Block: %d) - %s ---\n",
			chainInfo.Name, latestBlock, time.Now().Format("2006-01-02 15:04:05"))
		fmt.Printf("%-25s %-12s %-12s %-15s %-15s %s\n",
			"Subgraph", "Current", "Behind", "Sync Speed", "ETA", "Progress")

		// Check each subgraph for this chain
		for _, sg := range chainSubgraphs {
			// Get current block for this subgraph
			currentBlock, err := getCurrentBlock(sg.URL, query)
			if err != nil {
				log.Printf("Error for %s: %v", sg.Name, err)
				continue
			}

			// Store history for calculating sync speed
			now := time.Now()
			sg.LastCheckedBlocks = append(sg.LastCheckedBlocks, currentBlock)
			sg.LastCheckedTimes = append(sg.LastCheckedTimes, now)

			// Trim history to max entries
			if len(sg.LastCheckedBlocks) > sg.MaxHistoryEntries {
				sg.LastCheckedBlocks = sg.LastCheckedBlocks[1:]
				sg.LastCheckedTimes = sg.LastCheckedTimes[1:]
			}

			// Calculate sync metrics
			sg.CurrentBlock = currentBlock
			sg.LastBlock = latestBlock
			sg.BlocksBehind = latestBlock - currentBlock

			// Calculate sync speed and ETA only if we have at least 2 data points
			if len(sg.LastCheckedBlocks) >= 2 {
				firstIdx := 0
				lastIdx := len(sg.LastCheckedBlocks) - 1

				blockDiff := sg.LastCheckedBlocks[lastIdx] - sg.LastCheckedBlocks[firstIdx]
				timeDiff := sg.LastCheckedTimes[lastIdx].Sub(sg.LastCheckedTimes[firstIdx]).Minutes()

				if timeDiff > 0 {
					sg.SyncSpeed = float64(blockDiff) / timeDiff // blocks per minute

					if sg.SyncSpeed > 0 {
						etaMinutes := float64(sg.BlocksBehind) / sg.SyncSpeed
						sg.EstimatedTimeLeft = time.Duration(etaMinutes) * time.Minute
					} else {
						sg.EstimatedTimeLeft = 0
					}
				}
			}

			// Calculate progress percentage
			var progressPct float64
			if sg.BlocksBehind > 0 && latestBlock > 0 {
				progressPct = float64(currentBlock) / float64(latestBlock) * 100
			} else {
				progressPct = 100.0
			}

			// Format ETA for display
			etaDisplay := "Unknown"
			if sg.EstimatedTimeLeft > 0 {
				days := sg.EstimatedTimeLeft.Hours() / 24
				if days >= 1 {
					etaDisplay = fmt.Sprintf("%.1fd", days)
				} else if sg.EstimatedTimeLeft.Hours() >= 1 {
					etaDisplay = fmt.Sprintf("%.1fh", sg.EstimatedTimeLeft.Hours())
				} else {
					etaDisplay = fmt.Sprintf("%.0fm", sg.EstimatedTimeLeft.Minutes())
				}
			} else if sg.BlocksBehind == 0 {
				etaDisplay = "In sync"
			}

			// Display results
			fmt.Printf("%-25s %-12d %-12d %-15.2f %-15s %.2f%%\n",
				sg.Name,
				sg.CurrentBlock,
				sg.BlocksBehind,
				sg.SyncSpeed,
				etaDisplay,
				progressPct)
		}
	}
}

// getCurrentBlock fetches the current block from a subgraph
func getCurrentBlock(url, queryStr string) (int64, error) {
	// Create the HTTP client with a reasonable timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Parse the query string (which is already in JSON format)
	var queryObj map[string]interface{}
	if err := json.Unmarshal([]byte(queryStr), &queryObj); err != nil {
		return 0, fmt.Errorf("failed to parse query: %v", err)
	}

	// Prepare the request
	reqBody, err := json.Marshal(queryObj)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal query: %v", err)
	}

	// Create the request with proper headers
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Subgraph-Sync-Checker/1.0")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read the full response body
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response body: %v", err)
	}

	// Parse the response
	var response GraphQLResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return 0, fmt.Errorf("JSON decode error: %v, Raw response: %s", err, string(bodyBytes))
	}

	// Check for GraphQL errors
	if len(response.Errors) > 0 {
		return 0, fmt.Errorf("GraphQL error: %s", response.Errors[0].Message)
	}

	// Check if the response data structure is as expected
	if response.Data.Meta.Block.Number == 0 {
		// Try alternate response format - some GraphQL servers may have a different structure
		var alternateResponse map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &alternateResponse); err != nil {
			return 0, fmt.Errorf("failed to parse alternate response format: %v", err)
		}

		// Return the error for now
		return 0, fmt.Errorf("received zero block number, potential API structure mismatch")
	}

	return response.Data.Meta.Block.Number, nil
}

// getLatestBlockFromChain fetches the latest block from a blockchain
func getLatestBlockFromChain(chainName, rpcURL string) (int64, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Prepare the JSON-RPC request for eth_blockNumber
	reqBody, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_blockNumber",
		"params":  []string{},
		"id":      1,
	})
	if err != nil {
		return 0, err
	}

	// Make the request to the blockchain RPC endpoint
	resp, err := client.Post(rpcURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Parse the response
	var result struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	// Convert hex string to int64
	var blockNumber int64
	fmt.Sscanf(result.Result, "0x%x", &blockNumber)

	return blockNumber, nil
}
