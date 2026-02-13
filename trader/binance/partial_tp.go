package binance

import (
	"context"
	"fmt"
	"nofx/logger"
    "github.com/adshao/go-binance/v2/futures"
)

// SetPartialTakeProfit sets a partial take-profit order (Reduce-Only)
// This is used for Ladder Take Profit strategy
func (t *FuturesTrader) SetPartialTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	var side futures.SideType
	var posSide futures.PositionSideType

	if positionSide == "LONG" {
		side = futures.SideTypeSell
		posSide = futures.PositionSideTypeLong
	} else {
		side = futures.SideTypeBuy
		posSide = futures.PositionSideTypeShort
	}

    quantityStr, err := t.FormatQuantity(symbol, quantity)
    if err != nil {
        return err
    }

	// Use new Algo Order API
    // Note: ReduceOnly is implied for close orders in Hedge Mode usually, but better be safe
	_, err = t.client.NewCreateAlgoOrderService().
		Symbol(symbol).
		Side(side).
		PositionSide(posSide).
		Type(futures.AlgoOrderTypeTakeProfitMarket).
		TriggerPrice(t.FormatPrice(symbol, takeProfitPrice)).
        Quantity(quantityStr).
		WorkingType(futures.WorkingTypeContractPrice).
		ClosePosition(false). // Important: False for partial close
		ClientAlgoId(getBrOrderID()).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to set partial take-profit: %w", err)
	}

	logger.Infof("  ✓ Partial Take-profit set: %s %s @ %.4f (Qty: %s)", symbol, positionSide, takeProfitPrice, quantityStr)
	return nil
}
