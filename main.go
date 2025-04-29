package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	DefaultMaxHistoryEntries = 6
	CheckInterval            = 10 * time.Minute
	HTTPTimeout              = 10 * time.Second
)

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

type SubgraphInfo struct {
	Chain             string
	Name              string
	URL               string
	StartBlock        int64
	CurrentBlock      int64
	LastBlock         int64
	BlocksBehind      int64
	SyncSpeed         float64
	EstimatedTimeLeft time.Duration
	LastCheckedBlocks []int64
	LastCheckedTimes  []time.Time
	MaxHistoryEntries int
}

type ChainInfo struct {
	Name        string
	RpcURL      string
	LatestBlock int64
}

var (
	query = `{"query":"{_meta{block{number}}}"}`
	DefaultHTTPClient = &http.Client{Timeout: HTTPTimeout}
)

func main() {
	chains := initializeChains()
	subgraphs := initializeSubgraphs()

	checkSubgraphs(subgraphs, chains)

	ticker := time.NewTicker(CheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		checkSubgraphs(subgraphs, chains)
	}
}

func initializeChains() map[string]*ChainInfo {
	return map[string]*ChainInfo{
		"pulsechain": {
			Name:   "PulseChain",
			RpcURL: "https://rpc.pulsechain.com",
		},
	}
}

func initializeSubgraphs() []*SubgraphInfo {
	return []*SubgraphInfo{
		{
			Name:              "pDEX PulseChain Exchange 1",
			URL:               "https://graph.pulsechain.com/subgraphs/name/pulsechain/pulsex",
			StartBlock:        23287990,
			MaxHistoryEntries: DefaultMaxHistoryEntries,
			Chain:             "pulsechain",
		},
	}
}

func checkSubgraphs(subgraphs []*SubgraphInfo, chains map[string]*ChainInfo) {
	updateChainBlocks(chains)
	subgraphsByChain := groupSubgraphsByChain(subgraphs)

	for chainName, chainSubgraphs := range subgraphsByChain {
		chainInfo, ok := chains[chainName]
		if !ok {
			log.Printf("No chain info for %s", chainName)
			continue
		}
		processChainSubgraphs(chainInfo, chainSubgraphs)
	}
}

func updateChainBlocks(chains map[string]*ChainInfo) {
	for name, info := range chains {
		block, err := getLatestBlockFromChain(name, info.RpcURL)
		if err != nil {
			log.Printf("Chain %s error: %v", name, err)
			continue
		}
		info.LatestBlock = block
		log.Printf("Chain %s latest block: %d", name, block)
	}
}

func groupSubgraphsByChain(subgraphs []*SubgraphInfo) map[string][]*SubgraphInfo {
	group := make(map[string][]*SubgraphInfo)
	for _, sg := range subgraphs {
		group[sg.Chain] = append(group[sg.Chain], sg)
	}
	return group
}

func processChainSubgraphs(chainInfo *ChainInfo, subgraphs []*SubgraphInfo) {
	if chainInfo.LatestBlock == 0 {
		log.Printf("Skipping %s subgraphs, latest block = 0", chainInfo.Name)
		return
	}

	printHeader(chainInfo)

	for _, sg := range subgraphs {
		processSubgraph(sg, chainInfo.LatestBlock)
		printSubgraphStatus(sg)
	}
}

func printHeader(chainInfo *ChainInfo) {
	fmt.Printf("\n--- %s Subgraph Sync Status (Latest Block: %d) - %s ---\n",
		chainInfo.Name, chainInfo.LatestBlock, time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("%-25s %-12s %-12s %-12s %-15s %-15s %s\n",
		"Subgraph", "ChainBlock", "Subgraph", "Behind", "Sync Speed", "ETA", "Progress")
}

func processSubgraph(sg *SubgraphInfo, latestBlock int64) {
	current, err := getCurrentBlock(sg.URL, query)
	if err != nil {
		log.Printf("Error %s: %v", sg.Name, err)
		sg.CurrentBlock = 0
		sg.BlocksBehind = latestBlock - sg.StartBlock
		sg.SyncSpeed = 0
		sg.EstimatedTimeLeft = 0
		return
	}

	updateSubgraphHistory(sg, current)
	calculateSyncMetrics(sg, latestBlock)
}

func updateSubgraphHistory(sg *SubgraphInfo, currentBlock int64) {
	now := time.Now()
	sg.LastCheckedBlocks = append(sg.LastCheckedBlocks, currentBlock)
	sg.LastCheckedTimes = append(sg.LastCheckedTimes, now)

	if len(sg.LastCheckedBlocks) > sg.MaxHistoryEntries {
		sg.LastCheckedBlocks = sg.LastCheckedBlocks[1:]
		sg.LastCheckedTimes = sg.LastCheckedTimes[1:]
	}
}

func calculateSyncMetrics(sg *SubgraphInfo, latestBlock int64) {
	sg.CurrentBlock = sg.LastCheckedBlocks[len(sg.LastCheckedBlocks)-1]
	sg.LastBlock = latestBlock
	sg.BlocksBehind = latestBlock - sg.CurrentBlock

	if len(sg.LastCheckedBlocks) >= 2 {
		first := 0
		last := len(sg.LastCheckedBlocks) - 1

		blockDiff := sg.LastCheckedBlocks[last] - sg.LastCheckedBlocks[first]
		timeDiff := sg.LastCheckedTimes[last].Sub(sg.LastCheckedTimes[first]).Minutes()

		if timeDiff > 0 {
			sg.SyncSpeed = float64(blockDiff) / timeDiff
			if sg.SyncSpeed > 0 {
				etaMin := float64(sg.BlocksBehind) / sg.SyncSpeed
				sg.EstimatedTimeLeft = time.Duration(etaMin * float64(time.Minute))
			}
		}
	}
}

func printSubgraphStatus(sg *SubgraphInfo) {
	progressPct := calculateProgressPercentage(sg)
	etaDisplay := formatETA(sg)

	fmt.Printf("%-25s %-12d %-12s %-12d %-15.2f %-15s %.2f%%\n",
		sg.Name,
		sg.LastBlock,
		formatCurrentBlock(sg),
		sg.BlocksBehind,
		sg.SyncSpeed,
		etaDisplay,
		progressPct)
}

func calculateProgressPercentage(sg *SubgraphInfo) float64 {
	if sg.CurrentBlock == 0 || sg.StartBlock == 0 || sg.LastBlock <= sg.StartBlock {
		return 0.0
	}
	return float64(sg.CurrentBlock-sg.StartBlock) / float64(sg.LastBlock-sg.StartBlock) * 100
}

func formatETA(sg *SubgraphInfo) string {
	if sg.CurrentBlock == 0 {
		return "Error"
	}
	if sg.EstimatedTimeLeft <= 0 {
		if sg.BlocksBehind == 0 {
			return "In sync"
		}
		return "Unknown"
	}
	days := sg.EstimatedTimeLeft.Hours() / 24
	switch {
	case days >= 1:
		return fmt.Sprintf("%.1fd", days)
	case sg.EstimatedTimeLeft.Hours() >= 1:
		return fmt.Sprintf("%.1fh", sg.EstimatedTimeLeft.Hours())
	default:
		return fmt.Sprintf("%.0fm", sg.EstimatedTimeLeft.Minutes())
	}
}

func formatCurrentBlock(sg *SubgraphInfo) string {
	if sg.CurrentBlock == 0 {
		return "Error"
	}
	return fmt.Sprintf("%d", sg.CurrentBlock)
}

func getCurrentBlock(url, queryStr string) (int64, error) {
	var queryObj map[string]string
	if err := json.Unmarshal([]byte(queryStr), &queryObj); err != nil {
		return 0, fmt.Errorf("invalid GraphQL query: %v", err)
	}

	reqBody, err := json.Marshal(queryObj)
	if err != nil {
		return 0, fmt.Errorf("marshal query failed: %v", err)
	}

	resp, err := DefaultHTTPClient.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return 0, fmt.Errorf("HTTP error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read response failed: %v", err)
	}

	var response GraphQLResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return 0, fmt.Errorf("JSON error: %v", err)
	}
	if len(response.Errors) > 0 {
		return 0, fmt.Errorf("GraphQL errors: %v", response.Errors[0].Message)
	}
	if response.Data.Meta.Block.Number <= 0 {
		return 0, fmt.Errorf("invalid block number: %d", response.Data.Meta.Block.Number)
	}
	return response.Data.Meta.Block.Number, nil
}

func getLatestBlockFromChain(chainName, rpcURL string) (int64, error) {
	reqBody, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_blockNumber",
		"params":  []string{},
		"id":      1,
	})
	if err != nil {
		return 0, err
	}

	resp, err := DefaultHTTPClient.Post(rpcURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Result string `json:"result"`
		Error  struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	if result.Error.Message != "" {
		return 0, fmt.Errorf("RPC error: %s", result.Error.Message)
	}

	var block int64
	_, err = fmt.Sscanf(result.Result, "0x%x", &block)
	if err != nil {
		return 0, fmt.Errorf("parse block error: %v", err)
	}
	return block, nil
}
