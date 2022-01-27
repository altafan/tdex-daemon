package application

import (
	"encoding/json"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tdex-network/tdex-daemon/internal/core/domain"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
)

func checkForFeeAndMarketLowBalances(
	pubsubService ports.SecurePubSub,
	feeAccountBalance, feeAccountBalanceThreshold uint64,
	market *domain.Market, marketBalance Balance,
) {
	if feeAccountBalance < feeAccountBalanceThreshold {
		account := map[string]string{
			"name": FeeAccount,
		}
		publishAccountLowBalanceTopic(pubsubService, account, feeAccountBalance)
	}

	if market != nil {
		if marketBalance.BaseAmount <= uint64(market.FixedFee.BaseFee) ||
			marketBalance.QuoteAmount <= uint64(market.FixedFee.QuoteFee) {
			mktAccount := map[string]string{
				"name":        market.Name,
				"base_asset":  market.BaseAsset,
				"quote_asset": market.QuoteAsset,
			}
			publishAccountLowBalanceTopic(pubsubService, mktAccount, marketBalance)
		}
	}
}

// publishAccountLowBalanceTopic helper to publish an AccountLowBalance topic
// on the given pubsub service.
func publishAccountLowBalanceTopic(
	pubsub ports.SecurePubSub, account, balance interface{},
) {
	if pubsub == nil {
		return
	}

	topics := pubsub.TopicsByCode()
	topic := topics[AccountLowBalance]
	payload := map[string]interface{}{
		"event": map[string]interface{}{
			"code":  topic.Code(),
			"label": topic.Label(),
		},
		"account": account,
		"balance": balance,
	}
	message, _ := json.Marshal(payload)

	if err := pubsub.Publish(topic.Label(), string(message)); err != nil {
		log.WithError(err).Warnf(
			"an error occured while publishing message for topic %s", topic.Label(),
		)
	}
}

func publishMarketWithdrawTopic(
	pubsub ports.SecurePubSub,
	mkt *domain.Market, mktBalance, withdrewBalance Balance,
	destAddress, txid string,
) {
	if pubsub == nil {
		return
	}

	baseBalance := mktBalance.BaseAmount - withdrewBalance.BaseAmount
	quoteBalance := mktBalance.QuoteAmount - withdrewBalance.QuoteAmount

	topics := pubsub.TopicsByCode()
	topic := topics[AccountWithdraw]
	payload := map[string]interface{}{
		"event": map[string]interface{}{
			"code":  topic.Code(),
			"label": topic.Label(),
		},
		"market": map[string]string{
			"base_asset":  mkt.BaseAsset,
			"quote_asset": mkt.QuoteAsset,
			"name":        mkt.Name,
		},
		"amount_withdraw": map[string]interface{}{
			"base_amount":  withdrewBalance.BaseAmount,
			"quote_amount": withdrewBalance.QuoteAmount,
		},
		"receiving_address": destAddress,
		"txid":              txid,
		"balance": map[string]uint64{
			"base_balance":  baseBalance,
			"quote_balance": quoteBalance,
		},
	}
	message, _ := json.Marshal(payload)
	if err := pubsub.Publish(topic.Label(), string(message)); err != nil {
		log.WithError(err).Warnf(
			"an error occured while publishing message for topic %s",
			topic.Label(),
		)
	}
}

func publishFeeWithdrawTopic(
	pubsub ports.SecurePubSub,
	balance, withdrewBalance uint64,
	destAddress, txid, lbtcAsset string,
) {
	if pubsub == nil {
		return
	}

	lbtcBalance := balance - withdrewBalance

	topics := pubsub.TopicsByCode()
	topic := topics[AccountWithdraw]
	payload := map[string]interface{}{
		"event": map[string]interface{}{
			"code":  topic.Code(),
			"label": topic.Label(),
		},
		"fee": map[string]string{
			"lbtc_asset": lbtcAsset,
		},
		"amount_withdraw": map[string]interface{}{
			"lbtc_amount": withdrewBalance,
		},
		"receiving_address": destAddress,
		"txid":              txid,
		"balance": map[string]uint64{
			"lbtc_balance": lbtcBalance,
		},
	}
	message, _ := json.Marshal(payload)
	if err := pubsub.Publish(topic.Label(), string(message)); err != nil {
		log.WithError(err).Warnf(
			"an error occured while publishing message for topic %s",
			topic.Label(),
		)
	}
}

func publishTradeSettledTopic(
	pubsub ports.SecurePubSub,
	trade *domain.Trade, marketBaseAsset string,
	baseBalance, quoteBalance uint64,
) {
	if pubsub == nil {
		return
	}

	topics := pubsub.TopicsByCode()
	topic := topics[TradeSettled]
	payload := map[string]interface{}{
		"event": map[string]interface{}{
			"code":  topic.Code(),
			"label": topic.Label(),
		},
		"txid":                 trade.TxID,
		"settlement_timestamp": trade.SettlementTime,
		"settlement_date":      time.Unix(int64(trade.SettlementTime), 0).Format(time.UnixDate),
		"swap": map[string]interface{}{
			"amount_p": trade.SwapRequestMessage().GetAmountP(),
			"asset_p":  trade.SwapRequestMessage().GetAssetP(),
			"amount_r": trade.SwapRequestMessage().GetAmountR(),
			"asset_r":  trade.SwapRequestMessage().GetAssetR(),
		},
		"price": map[string]string{
			"base_price":  trade.MarketPrice.BasePrice.String(),
			"quote_price": trade.MarketPrice.QuotePrice.String(),
		},
		"market": map[string]string{
			"base_asset":  marketBaseAsset,
			"quote_asset": trade.MarketQuoteAsset,
		},
		"balance": map[string]uint64{
			"base_balance":  baseBalance,
			"quote_balance": quoteBalance,
		},
	}
	message, _ := json.Marshal(payload)
	if err := pubsub.Publish(topic.Label(), string(message)); err != nil {
		log.WithError(err).Warnf(
			"an error occured while publishing message for topic %s",
			topic.Label(),
		)
	}
}
