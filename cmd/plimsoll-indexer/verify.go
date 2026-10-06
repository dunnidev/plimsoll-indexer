package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/plimsoll-protocol/plimsoll-indexer/internal/horizon"
)

type apiAssetResp struct {
	SAC    string `json:"sac"`
	Code   string `json:"code"`
	Issuer string `json:"issuer"`
	Supply *struct {
		Amount        string `json:"amount"`
		Ledger        uint32 `json:"ledger"`
		BreakdownHash string `json:"breakdown_hash"`
	} `json:"supply"`
}

type apiNetworkResp struct {
	HorizonURL string `json:"horizon_url"`
}

func runVerifyCommand(args []string) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	sacFlag := fs.String("sac", "", "Stellar Asset Contract (SAC) address")
	indexerFlag := fs.String("indexer", "http://localhost:8080", "Indexer API base URL")
	horizonFlag := fs.String("horizon", "", "Horizon API base URL (optional)")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *sacFlag == "" {
		fmt.Fprintln(os.Stderr, "Error: --sac flag is required")
		fs.Usage()
		return 1
	}

	indexerURL := strings.TrimRight(*indexerFlag, "/")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	httpClient := &http.Client{Timeout: 15 * time.Second}

	// 1. Fetch asset details from indexer
	assetReq, err := http.NewRequestWithContext(ctx, http.MethodGet, indexerURL+"/v1/assets/"+*sacFlag, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
		return 1
	}
	assetResp, err := httpClient.Do(assetReq)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching asset from indexer: %v\n", err)
		return 1
	}
	defer assetResp.Body.Close()

	if assetResp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Error: indexer returned status %d for asset %s\n", assetResp.StatusCode, *sacFlag)
		return 1
	}

	var asset apiAssetResp
	if err := json.NewDecoder(assetResp.Body).Decode(&asset); err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding asset response: %v\n", err)
		return 1
	}

	if asset.Supply == nil || asset.Supply.BreakdownHash == "" {
		fmt.Fprintf(os.Stderr, "Error: no supply snapshot found for asset %s\n", *sacFlag)
		return 1
	}

	// 2. Fetch breakdown from indexer
	breakdownReq, err := http.NewRequestWithContext(ctx, http.MethodGet, indexerURL+"/v1/breakdowns/"+asset.Supply.BreakdownHash, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating breakdown request: %v\n", err)
		return 1
	}
	breakdownResp, err := httpClient.Do(breakdownReq)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching breakdown from indexer: %v\n", err)
		return 1
	}
	defer breakdownResp.Body.Close()

	if breakdownResp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Error: indexer returned status %d for breakdown %s\n", breakdownResp.StatusCode, asset.Supply.BreakdownHash)
		return 1
	}

	breakdownBytes, err := io.ReadAll(breakdownResp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading breakdown body: %v\n", err)
		return 1
	}

	// Verify SHA-256 hash match
	computedHash := sha256.Sum256(breakdownBytes)
	computedHashHex := hex.EncodeToString(computedHash[:])
	if computedHashHex != asset.Supply.BreakdownHash {
		fmt.Fprintf(os.Stderr, "Error: breakdown hash mismatch! Expected %s, got %s\n", asset.Supply.BreakdownHash, computedHashHex)
		return 1
	}

	var snapshotBreakdown horizon.Breakdown
	if err := json.Unmarshal(breakdownBytes, &snapshotBreakdown); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing breakdown JSON: %v\n", err)
		return 1
	}

	// 3. Determine Horizon URL
	horizonURL := *horizonFlag
	if horizonURL == "" {
		netReq, err := http.NewRequestWithContext(ctx, http.MethodGet, indexerURL+"/v1/network", nil)
		if err == nil {
			if netResp, err := httpClient.Do(netReq); err == nil {
				var netInfo apiNetworkResp
				if json.NewDecoder(netResp.Body).Decode(&netInfo) == nil && netInfo.HorizonURL != "" {
					horizonURL = netInfo.HorizonURL
				}
				netResp.Body.Close()
			}
		}
	}
	if horizonURL == "" {
		horizonURL = "https://horizon.stellar.org"
	}

	// 4. Fetch current supply from Horizon
	hzClient := horizon.NewClient(horizonURL)
	curBreakdown, err := hzClient.Supply(ctx, asset.Code, asset.Issuer)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching supply from Horizon (%s): %v\n", horizonURL, err)
		return 1
	}

	// 5. Compare & print diff table
	return printVerificationResult(os.Stdout, asset.Code, asset.SAC, snapshotBreakdown, curBreakdown)
}

func printVerificationResult(out io.Writer, code, sac string, snapshot, current horizon.Breakdown) int {
	fmt.Fprintf(out, "Asset: %s (SAC: %s)\n", code, sac)
	fmt.Fprintf(out, "Snapshot Ledger: %d | Current Horizon Ledger: %d\n", snapshot.Ledger, current.Ledger)
	fmt.Fprintf(out, "Breakdown Hash: %s (SHA-256 match: OK)\n\n", snapshot.Hash())

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Bucket\tSnapshot\tCurrent Horizon\tDiff")
	fmt.Fprintln(w, "------\t--------\t---------------\t----")

	buckets := []struct {
		name string
		snap string
		cur  string
	}{
		{"Authorized", snapshot.Authorized, current.Authorized},
		{"AuthorizedToMaintainLiabilities", snapshot.AuthorizedToMaintainLiabilities, current.AuthorizedToMaintainLiabilities},
		{"Unauthorized", snapshot.Unauthorized, current.Unauthorized},
		{"ClaimableBalances", snapshot.ClaimableBalances, current.ClaimableBalances},
		{"LiquidityPools", snapshot.LiquidityPools, current.LiquidityPools},
		{"Contracts", snapshot.Contracts, current.Contracts},
	}

	match := true

	calcDiff := func(snapStr, curStr string) (string, bool) {
		snapInt, _ := new(big.Int).SetString(snapStr, 10)
		if snapInt == nil {
			snapInt = new(big.Int)
		}
		curInt, _ := new(big.Int).SetString(curStr, 10)
		if curInt == nil {
			curInt = new(big.Int)
		}
		diff := new(big.Int).Sub(curInt, snapInt)
		isSame := diff.Sign() == 0
		var diffStr string
		if diff.Sign() > 0 {
			diffStr = "+" + diff.String()
		} else {
			diffStr = diff.String()
		}
		return diffStr, isSame
	}

	for _, b := range buckets {
		diffStr, isSame := calcDiff(b.snap, b.cur)
		if !isSame {
			match = false
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", b.name, b.snap, b.cur, diffStr)
	}

	fmt.Fprintln(w, "------\t--------\t---------------\t----")
	totDiffStr, totSame := calcDiff(snapshot.Total, current.Total)
	if !totSame {
		match = false
	}
	fmt.Fprintf(w, "Total\t%s\t%s\t%s\n", snapshot.Total, current.Total, totDiffStr)
	w.Flush()

	fmt.Fprintln(out)
	if match {
		fmt.Fprintln(out, "Result: MATCH")
		return 0
	}
	fmt.Fprintln(out, "Result: MISMATCH")
	return 1
}
