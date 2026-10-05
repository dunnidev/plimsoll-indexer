// Package horizon computes the circulating supply of a Stellar-issued asset
// from Horizon's /assets endpoint.
package horizon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrAssetNotFound means Horizon has no record of the asset (no trustlines yet).
var ErrAssetNotFound = errors.New("asset not found on horizon")

// Breakdown is the per-bucket supply. Its canonical JSON encoding is hashed
// and the hash is posted on-chain with the total, so anyone can recompute it.
// Amounts are integer strings in the asset's smallest unit (7 decimals).
type Breakdown struct {
	Asset                           string `json:"asset"`
	Ledger                          uint32 `json:"ledger"`
	Authorized                      string `json:"authorized"`
	AuthorizedToMaintainLiabilities string `json:"authorized_to_maintain_liabilities"`
	Unauthorized                    string `json:"unauthorized"`
	ClaimableBalances               string `json:"claimable_balances"`
	LiquidityPools                  string `json:"liquidity_pools"`
	Contracts                       string `json:"contracts"`
	Total                           string `json:"total"`
}

// Canonical returns the exact bytes that are hashed.
func (b Breakdown) Canonical() []byte {
	out, _ := json.Marshal(b) // struct of strings and a uint32 cannot fail
	return out
}

// Hash is the hex sha256 of Canonical().
func (b Breakdown) Hash() string {
	sum := sha256.Sum256(b.Canonical())
	return hex.EncodeToString(sum[:])
}

// TotalInt returns Total as a big.Int.
func (b Breakdown) TotalInt() *big.Int {
	n, _ := new(big.Int).SetString(b.Total, 10)
	return n
}

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 20 * time.Second},
	}
}

type AssetRecord struct {
	Balances struct {
		Authorized                      string `json:"authorized"`
		AuthorizedToMaintainLiabilities string `json:"authorized_to_maintain_liabilities"`
		Unauthorized                    string `json:"unauthorized"`
	} `json:"balances"`
	ClaimableBalancesAmount string `json:"claimable_balances_amount"`
	LiquidityPoolsAmount    string `json:"liquidity_pools_amount"`
	ContractsAmount         string `json:"contracts_amount"`
}

type root struct {
	HistoryLatestLedger uint32 `json:"history_latest_ledger"`
}

type account struct {
	HomeDomain string `json:"home_domain"`
}

// LatestLedger returns the newest ledger Horizon has ingested.
func (c *Client) LatestLedger(ctx context.Context) (uint32, error) {
	var r root
	if err := c.get(ctx, "/", &r); err != nil {
		return 0, fmt.Errorf("horizon root: %w", err)
	}
	return r.HistoryLatestLedger, nil
}

// HomeDomain returns the issuer account's home_domain ("" if unset).
func (c *Client) HomeDomain(ctx context.Context, issuer string) (string, error) {
	var a account
	if err := c.get(ctx, "/accounts/"+url.PathEscape(issuer), &a); err != nil {
		return "", fmt.Errorf("horizon account %s: %w", issuer, err)
	}
	return a.HomeDomain, nil
}

// Supply fetches the asset record and sums every bucket that is a claim on
// the issuer: trustline balances in all authorization states, claimable
// balances, liquidity-pool reserves and balances held by contracts.
func (c *Client) Supply(ctx context.Context, code, issuer string) (Breakdown, error) {
	ledger, err := c.LatestLedger(ctx)
	if err != nil {
		return Breakdown{}, err
	}
	q := url.Values{"asset_code": {code}, "asset_issuer": {issuer}}
	var page struct {
		Embedded struct {
			Records []AssetRecord `json:"records"`
		} `json:"_embedded"`
	}
	if err := c.get(ctx, "/assets?"+q.Encode(), &page); err != nil {
		return Breakdown{}, fmt.Errorf("horizon assets %s:%s: %w", code, issuer, err)
	}
	if len(page.Embedded.Records) == 0 {
		return Breakdown{}, ErrAssetNotFound
	}
	return BreakdownFromRecord(code+":"+issuer, ledger, page.Embedded.Records[0])
}

// BreakdownFromRecord converts Horizon's decimal strings to integer units.
func BreakdownFromRecord(asset string, ledger uint32, r AssetRecord) (Breakdown, error) {
	type bucket struct {
		name string
		in   string
		out  *string
	}
	b := Breakdown{Asset: asset, Ledger: ledger}
	fields := []bucket{
		{"authorized", r.Balances.Authorized, &b.Authorized},
		{"authorized_to_maintain_liabilities", r.Balances.AuthorizedToMaintainLiabilities, &b.AuthorizedToMaintainLiabilities},
		{"unauthorized", r.Balances.Unauthorized, &b.Unauthorized},
		{"claimable_balances_amount", r.ClaimableBalancesAmount, &b.ClaimableBalances},
		{"liquidity_pools_amount", r.LiquidityPoolsAmount, &b.LiquidityPools},
		{"contracts_amount", r.ContractsAmount, &b.Contracts},
	}
	total := new(big.Int)
	for _, f := range fields {
		n, err := ParseAmount(f.in)
		if err != nil {
			return Breakdown{}, fmt.Errorf("%s: %w", f.name, err)
		}
		*f.out = n.String()
		total.Add(total, n)
	}
	b.Total = total.String()
	return b, nil
}

// ParseAmount converts a Horizon decimal string ("12.3400000") to an integer
// count of the smallest unit (7 decimals). An empty string is zero.
func ParseAmount(s string) (*big.Int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return new(big.Int), nil
	}
	if strings.HasPrefix(s, "-") {
		return nil, fmt.Errorf("negative amount %q", s)
	}
	whole, frac, _ := strings.Cut(s, ".")
	if len(frac) > 7 {
		return nil, fmt.Errorf("amount %q has more than 7 decimals", s)
	}
	frac += strings.Repeat("0", 7-len(frac))
	if whole == "" {
		whole = "0"
	}
	n, ok := new(big.Int).SetString(whole+frac, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount %q", s)
	}
	return n, nil
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrAssetNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
