package main

import (
	"context"
	"fmt"
	"github.com/lightningnetwork/lnd/lnrpc"
	"math/rand"
	"time"
)

type Activity struct {
	target  int
	aliases map[string]*alias
}

func NewActivity(n int, aliases map[string]*alias) *Activity {
	return &Activity{
		target:  n,
		aliases: aliases,
	}
}

const MIN_INVOICE = 1
const MAX_INVOICE = 12000

func (a *Activity) Run() {
	go (func() {
		indexedAliases := make([]*alias, 0)

		for _, o := range a.aliases {
			indexedAliases = append(indexedAliases, o)
		}

		for i := 0; i < a.target; i++ {
			time.Sleep(2000 * time.Millisecond)
			srcindex := rand.Intn(len(a.aliases))
			var destindex int
			for {
				destindex = rand.Intn(len(indexedAliases))
				if destindex != srcindex {
					break
				}
			}
			src := indexedAliases[srcindex]
			dest := indexedAliases[destindex]

			destrpc := grpcClient(dest)

			ctx := context.Background()
			destInvResp, err := destrpc.AddInvoice(ctx, &lnrpc.Invoice{
				Value: int64(rand.Intn(MAX_INVOICE) + MIN_INVOICE),
				Memo:  fmt.Sprintf("random invoice from %s, to %s", *src.Name, *dest.Name),
			})
			if err != nil {
				continue
			}

			srcrpc := grpcClient(src)

			_, err = srcrpc.SendPaymentSync(ctx, &lnrpc.SendRequest{
				PaymentRequest: destInvResp.PaymentRequest,
			})
			if err != nil {
				continue
			}

		}

	})()
}
