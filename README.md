# 🔄 SubGraph Sync Monitor

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

A lightweight, powerful tool to monitor the synchronization status of blockchain subgraphs. Keep track of indexing progress, sync speeds, and estimated time to completion across multiple subgraphs and chains.

![Terminal Screenshot](https://github.com/nikola43/subgraph-sync-monitor/raw/main/screenshot.png)

## 🌟 Features

- **Multi-Chain Support**: Monitor subgraphs across different blockchains (Ethereum, PulseChain, etc.)
- **Real-Time Sync Metrics**: Track each subgraph's current block, blocks behind, and sync percentage
- **Smart ETA Calculation**: Adaptive sync speed calculation based on historical data
- **Periodic Checking**: Automatically checks sync status at configurable intervals
- **Formatted Reporting**: Clean, tabular output for easy monitoring

## 📋 Example Output

```
--- PulseChain Subgraph Sync Status (Latest Block: 18457234) - 2025-03-28 15:04:05 ---
Subgraph                  Current      Behind       Sync Speed      ETA             Progress
PulseX PulseChain V2      18452341     4893         128.45          38m             99.73%
```

## 🚀 Quick Start

### Prerequisites

- Go 1.16 or higher
- Access to blockchain RPC endpoints

### Installation

```bash
# Clone the repository
git clone https://github.com/nikola43/subgraph-sync-monitor.git
cd subgraph-sync-monitor

# Build the application
go build -o subgraph-monitor .
```

### Running

```bash
# Run with default settings
./subgraph-monitor

# Or run in the background
nohup ./subgraph-monitor > monitor.log 2>&1 &
```

## ⚙️ Configuration

Edit the `main.go` file to configure your chains and subgraphs:

```go
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
    // Add more subgraphs as needed
}
```

## 📈 How It Works

1. The application queries RPC endpoints to get the latest block for each chain
2. It then queries each subgraph to get its current indexed block
3. Sync metrics are calculated, including:
   - Blocks behind (latest chain block - current subgraph block)
   - Sync speed (blocks indexed per minute)
   - Estimated time to completion
   - Overall sync percentage
4. Results are displayed in a formatted table
5. The process repeats based on the configured interval (default: 10 minutes)

## 🔧 Advanced Usage

### Customizing Check Interval

Modify the ticker duration in `main.go`:

```go
// Check every 5 minutes instead of 10
ticker := time.NewTicker(5 * time.Minute)
```

### Adding More Chains and Subgraphs

Simply add more entries to the `chains` and `subgraphs` data structures in `main.go`.

### Modifying ETA Calculation

Adjust the `MaxHistoryEntries` to change how many data points are used for calculating sync speed:

```go
// Use more data points for smoother ETA calculation
{
    Name:              "My Subgraph",
    URL:               "https://graph.example.com/subgraphs/name/my-subgraph",
    MaxHistoryEntries: 12, // Use last 12 checks instead of default 6
    Chain:             "ethereum",
}
```

## 📝 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🤝 Contributing

Contributions, issues, and feature requests are welcome! Feel free to check the [issues page](https://github.com/nikola43/subgraph-sync-monitor/issues).

## 💻 Author

- **Your Name** - [GitHub Profile](https://github.com/nikola43)

---

Made with ❤️ for the blockchain community
