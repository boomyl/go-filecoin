package net

import (
	"context"
	"fmt"

	"github.com/ipfs/go-block-format"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/types"
)

// Fetcher defines an interface that may be used to fetch data from the network.
type Fetcher interface {
	FetchTipSets(ctx context.Context, tsKey types.TipSetKey, recur int) ([]types.TipSet, error)
	GetBlocks(ctx context.Context, cids []cid.Cid) ([]*types.Block, error)
}

// BitswapFetcher is used to fetch data over the network.  It is implemented with
// a persistent bitswap session on a networked blockservice.
type BitswapFetcher struct {
	// session is a bitswap session that enables efficient transfer.
	session   *bserv.Session
	validator consensus.BlockSyntaxValidator
}

// NewBitswapFetcher returns a BitswapFetcher wired up to the input BlockService and a newly
// initialized persistent session of the block service.
func NewBitswapFetcher(ctx context.Context, bsrv bserv.BlockService, bv consensus.BlockSyntaxValidator) *BitswapFetcher {
	return &BitswapFetcher{
		session:   bserv.NewSession(ctx, bsrv),
		validator: bv,
	}
}

// FetchTipSets fetchs the tipset at `tsKey` from the network using the fetchers bitswap session.
// FetchTipSets will fetch `recur` partens of `tsKey`. FetchTipSets does not return partial results.
func (bsf *BitswapFetcher) FetchTipSets(ctx context.Context, tsKey types.TipSetKey, recur int) ([]types.TipSet, error) {
	var out []types.TipSet
	cur := tsKey
	for i := 0; i < recur; i++ {
		res, err := bsf.GetBlocks(ctx, cur.ToSlice())
		if err != nil {
			return nil, err
		}

		ts, err := types.NewTipSet(res...)
		if err != nil {
			return nil, err
		}

		cur, err = ts.Parents()
		if err != nil {
			return nil, err
		}

		out = append(out, ts)
	}

	return out, nil
}

// GetBlocks fetches the blocks with the given cids from the network using the
// BitswapFetcher's bitswap session.
func (bsf *BitswapFetcher) GetBlocks(ctx context.Context, cids []cid.Cid) ([]*types.Block, error) {
	var unsanitized []blocks.Block
	for b := range bsf.session.GetBlocks(ctx, cids) {
		unsanitized = append(unsanitized, b)
	}

	if len(unsanitized) < len(cids) {
		var err error
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = errors.Wrap(ctxErr, "failed to fetch all requested blocks")
		} else {
			err = errors.New("failed to fetch all requested blocks")
		}
		return nil, err
	}

	var blocks []*types.Block
	for _, u := range unsanitized {
		block, err := types.DecodeBlock(u.RawData())
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("fetched data (cid %s) was not a block", u.Cid().String()))
		}

		// reject blocks that are syntactically invalid.
		if err := bsf.validator.ValidateSyntax(ctx, block); err != nil {
			continue
		}

		blocks = append(blocks, block)
	}
	return blocks, nil
}
