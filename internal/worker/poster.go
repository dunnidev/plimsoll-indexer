package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/stellar/go-stellar-sdk/clients/rpcclient"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/dunnidev/plimsoll-indexer/internal/horizon"
	"github.com/dunnidev/plimsoll-indexer/internal/soroban"
	"github.com/dunnidev/plimsoll-indexer/internal/store"
)

// Poster computes each listed asset's supply from Horizon and posts it to the
// coverage ledger when it changed, or when the last post is older than
// RepostAfter (so is_covered freshness checks keep passing).
type Poster struct {
	Horizon     *horizon.Client
	RPC         *rpcclient.Client
	Invoker     *soroban.Invoker
	Store       *store.Store
	LedgerID    string
	RepostAfter time.Duration
	Log         *slog.Logger
	Now         func() time.Time
}

func (p *Poster) Run(ctx context.Context) error {
	assets, err := p.Store.ListAssets(ctx)
	if err != nil {
		return fmt.Errorf("list assets: %w", err)
	}
	var errs []error
	for _, a := range assets {
		if err := p.postOne(ctx, a.SAC, a.Code, a.Issuer); err != nil {
			errs = append(errs, fmt.Errorf("%s:%s: %w", a.Code, a.Issuer, err))
		}
	}
	return errors.Join(errs...)
}

func (p *Poster) postOne(ctx context.Context, sac, code, issuer string) error {
	b, err := p.Horizon.Supply(ctx, code, issuer)
	if errors.Is(err, horizon.ErrAssetNotFound) {
		p.Log.Debug("asset has no holders yet", "code", code)
		return nil
	}
	if err != nil {
		return err
	}

	last, err := p.Store.LatestSupply(ctx, sac)
	if err != nil {
		return fmt.Errorf("latest supply: %w", err)
	}
	if last != nil {
		unchanged := last.Amount.Cmp(b.TotalInt()) == 0
		recent := p.Now().Sub(last.PostedAt) < p.RepostAfter
		if unchanged && recent {
			return nil
		}
		if b.Ledger <= last.Ledger {
			return nil // Horizon has not moved past our last snapshot yet.
		}
	}

	// The contract rejects a ledger newer than the one it is executing in.
	rpcLatest, err := p.RPC.GetLatestLedger(ctx)
	if err != nil {
		return fmt.Errorf("getLatestLedger: %w", err)
	}
	if b.Ledger > rpcLatest.Sequence {
		b.Ledger = rpcLatest.Sequence
	}

	hash := b.Hash()
	if err := p.Store.SaveBreakdown(ctx, hash, sac, b.Canonical()); err != nil {
		return fmt.Errorf("save breakdown: %w", err)
	}

	sacVal, err := soroban.AddressVal(sac)
	if err != nil {
		return err
	}
	posterVal, err := soroban.AddressVal(p.Invoker.Signer.Address())
	if err != nil {
		return err
	}
	amountVal, err := soroban.I128Val(b.TotalInt())
	if err != nil {
		return err
	}
	hashVal, err := soroban.BytesN32Val(hash)
	if err != nil {
		return err
	}
	args := []xdr.ScVal{posterVal, sacVal, amountVal, soroban.U32Val(b.Ledger), hashVal}
	res, err := p.Invoker.Invoke(ctx, p.LedgerID, "post_supply", args)
	if err != nil {
		return fmt.Errorf("post_supply: %w", err)
	}
	p.Log.Info("posted supply", "code", code, "amount", b.Total, "at_ledger", b.Ledger, "tx", res.TxHash)
	return nil
}
