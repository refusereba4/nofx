package risk_control

import (
	"fmt"
	"math"
	"nofx/logger"
	"nofx/manager"
	"nofx/store"
	"nofx/trader/binance"
	"strconv"
	"time"
)

type positionProgress struct {
	InitialAmount   float64
	LastAmount      float64
	LastEntryPrice  float64
	FilledBatches   int // 0~len(plan)
	EnforcedOnce    bool
	MultiSLDisabled bool // fallback to single-SL mode if exchange rejects dual-SL
}

type tpBatch struct {
	TargetROE float64 // decimal, e.g. 0.005 = +0.5%
	PosPct    float64 // decimal, e.g. 0.2 = 20%
}

type stagedStopLoss struct {
	InitialROE  float64 // decimal, e.g. -0.016 = -1.6%
	AfterTP1ROE float64
	AfterTP2ROE float64
}

// RiskWatcher continuously monitors positions and enforces risk control rules
type RiskWatcher struct {
	traderManager *manager.TraderManager
	interval      time.Duration
	// Per trader active symbols in previous scan, used to cleanup newly-closed positions only.
	activeHistory map[string]map[string]bool
	// Tracks ladder progress for each trader/symbol/side.
	progress map[string]*positionProgress
}

// NewRiskWatcher creates a new risk watcher
func NewRiskWatcher(tm *manager.TraderManager) *RiskWatcher {
	return &RiskWatcher{
		traderManager: tm,
		interval:      10 * time.Second, // Scan every 10 seconds
		activeHistory: make(map[string]map[string]bool),
		progress:      make(map[string]*positionProgress),
	}
}

// Start starts the risk watcher loop
func (w *RiskWatcher) Start() {
	logger.Info("🛡️ Risk Control Service started (interval: 10s)")
	go func() {
		for {
			w.scanAndEnforce()
			time.Sleep(w.interval)
		}
	}()
}

// scanAndEnforce scans all traders and enforces risk rules
func (w *RiskWatcher) scanAndEnforce() {
	traders := w.traderManager.GetAllTraders()

	for traderID, t := range traders {
		status := t.GetStatus()
		if isRunning, ok := status["is_running"].(bool); ok && !isRunning {
			// Only running traders should manage live risk orders.
			// Prevent stopped traders from modifying the same account orders.
			continue
		}

		// Get underlying trader
		underlying := t.GetUnderlyingTrader()

		// Type assert to Binance FuturesTrader
		binanceTrader, ok := underlying.(*binance.FuturesTrader)
		if !ok {
			continue // Skip non-Binance traders
		}

		// Get active positions only (GetPositions filters out positionAmt == 0)
		positions, err := binanceTrader.GetPositionsForRiskWatcher()
		if err != nil {
			logger.Warnf("🛡️ Failed to get positions for trader %s: %v", t.GetName(), err)
			continue
		}

		prevActive, ok := w.activeHistory[traderID]
		if !ok {
			prevActive = make(map[string]bool)
		}
		currentActive := make(map[string]bool)

		// 1. Process active positions only
		for _, pos := range positions {
			symbol := pos["symbol"].(string)
			amt := pos["positionAmt"].(float64)
			entryPrice := pos["entryPrice"].(float64)
			leverage, _ := pos["leverage"].(float64)

			if amt == 0 {
				continue
			}

			currentActive[symbol] = true

			side := "LONG"
			if amt < 0 {
				side = "SHORT"
				amt = -amt
			}

			riskCfg := t.GetRiskControlConfig()
			batches, compactBatches, slCfg := normalizeExitConfig(riskCfg)

			progress := w.updatePositionProgress(traderID, symbol, side, amt, entryPrice, batches)
			if err := w.enforceRules(binanceTrader, symbol, side, amt, entryPrice, leverage, progress, batches, compactBatches, slCfg); err != nil {
				logger.Warnf("🛡️ Failed to enforce rules for %s %s: %v", symbol, side, err)
			}
		}

		// 2. Cleanup only symbols that were active in previous scan but now closed
		cleanupCandidates := make(map[string]bool)
		for symbol := range prevActive {
			cleanupCandidates[symbol] = true
		}

		// Fallback cleanup path:
		// after restart prevActive may be empty, but orphan risk orders can still exist.
		riskOrderSymbols, err := binanceTrader.GetRiskOrderSymbolsForRiskWatcher()
		if err != nil {
			logger.Warnf("🛡️ Failed to list risk-order symbols for trader %s: %v", t.GetName(), err)
		} else {
			for symbol := range riskOrderSymbols {
				cleanupCandidates[symbol] = true
			}
		}

		for symbol := range cleanupCandidates {
			if currentActive[symbol] {
				continue
			}
			w.cleanupOrphanOrders(binanceTrader, symbol)
			w.clearSymbolProgress(traderID, symbol)
		}

		// 3. Save active set for next scan
		w.activeHistory[traderID] = currentActive
	}
}

// cleanupOrphanOrders checks for and removes SL/TP orders when position is closed
func (w *RiskWatcher) cleanupOrphanOrders(t *binance.FuturesTrader, symbol string) {
	openOrders, err := t.GetOpenOrdersForRiskWatcher(symbol)
	if err != nil {
		logger.Warnf("🛡️ Failed to get open orders for closed symbol %s: %v", symbol, err)
		return
	}

	orphanCount := 0
	for _, o := range openOrders {
		if isRiskOrderType(o.Type) {
			orphanCount++
		}
	}

	if orphanCount > 0 {
		logger.Infof("🛡️ Found %d orphan SL/TP orders for closed position %s -> Canceling", orphanCount, symbol)
		if err := t.CancelStopOrders(symbol); err != nil {
			logger.Warnf("🛡️ Failed to cancel orphan orders for %s: %v", symbol, err)
		} else {
			logger.Infof("  ✓ Orphan orders cleaned up for %s", symbol)
		}
	}
}

// enforceRules enforcing SL and TP rules using ROE Logic and Cumulative Ladder
func (w *RiskWatcher) enforceRules(
	t *binance.FuturesTrader,
	symbol, side string,
	amount, entryPrice, leverage float64,
	progress *positionProgress,
	batches []tpBatch,
	compactBatches []tpBatch,
	slCfg stagedStopLoss,
) error {
	openOrders, err := t.GetOpenOrdersForRiskWatcher(symbol)
	if err != nil {
		return err
	}

	slCount := 0
	tpCount := 0
	var slPrices []float64

	for _, o := range openOrders {
		if o.PositionSide != side {
			continue
		}
		if o.Type == "STOP_MARKET" || o.Type == "STOP" {
			slCount++
			if o.StopPrice > 0 {
				slPrices = append(slPrices, o.StopPrice)
			}
		}
		if o.Type == "TAKE_PROFIT_MARKET" || o.Type == "TAKE_PROFIT" {
			tpCount++
		}
	}

	if leverage < 1 {
		leverage = 1
	}

	type TPOrder struct {
		BatchIndex int
		Qty        float64
		Price      float64
	}
	var ordersToPlace []TPOrder
	initialAmount := amount
	filledBatches := 0
	if progress != nil {
		if progress.InitialAmount > 0 {
			initialAmount = progress.InitialAmount
		}
		filledBatches = progress.FilledBatches
	}

	// Recovery path after process restart:
	// only recover when TP count strongly indicates a partially-filled staged ladder.
	// tpCount==1 is ambiguous (fresh new position often has a single AI TP), so do NOT recover from it.
	if progress != nil && !progress.EnforcedOnce && filledBatches == 0 && tpCount >= 2 && tpCount < len(batches) {
		recoveredFilled := len(batches) - tpCount
		if recoveredFilled > 0 {
			remainRatio := remainingRatioAfterFilled(batches, recoveredFilled)
			if remainRatio > 0 {
				initialAmount = amount / remainRatio
			}
			progress.InitialAmount = initialAmount
			progress.FilledBatches = recoveredFilled
			filledBatches = recoveredFilled
			// Existing TP ladder is already on exchange; keep it and only reconcile staged SL.
			progress.EnforcedOnce = true
			logger.Infof("🛡️ %s %s recovered ladder stage after restart: tpCount=%d => filled=%d, initial=%.4f current=%.4f",
				symbol, side, tpCount, recoveredFilled, initialAmount, amount)
		}
	}

	if filledBatches < 0 {
		filledBatches = 0
	}
	if filledBatches > len(batches) {
		filledBatches = len(batches)
	}

	// Safety: if progress says some TP batches are already filled but current amount
	// does not match that stage anymore, treat as a new position cycle and reset state.
	if progress != nil && filledBatches > 0 && progress.InitialAmount > 0 {
		actualRatio := amount / progress.InitialAmount
		expectedRemain := remainingRatioAfterFilled(batches, filledBatches)
		if actualRatio > expectedRemain+0.08 {
			logger.Infof("🛡️ %s %s detected new cycle (ratio mismatch: actual=%.4f expected<=%.4f), reset ladder progress",
				symbol, side, actualRatio, expectedRemain)
			progress.InitialAmount = amount
			progress.LastAmount = amount
			progress.FilledBatches = 0
			progress.EnforcedOnce = false
			progress.MultiSLDisabled = false
			initialAmount = amount
			filledBatches = 0
		}
	}

	buildOrders := func(plan []tpBatch, filled int) []TPOrder {
		useFilled := filled
		if useFilled < 0 {
			useFilled = 0
		}
		if useFilled > len(plan) {
			useFilled = len(plan)
		}

		// Scale remaining batch quantities to current amount.
		// This avoids re-creating already filled tiers and prevents TP re-hanging loops.
		sumFilledPct := 0.0
		for i := 0; i < useFilled; i++ {
			sumFilledPct += plan[i].PosPct
		}
		expectedRemaining := initialAmount * (1 - sumFilledPct)
		scale := 1.0
		if expectedRemaining > 0 {
			scale = amount / expectedRemaining
		}
		if scale <= 0 {
			scale = 1.0
		}

		remainingToAllocate := amount
		var out []TPOrder
		for i := useFilled; i < len(plan); i++ {
			rawQty := initialAmount * plan[i].PosPct * scale
			if rawQty > remainingToAllocate {
				rawQty = remainingToAllocate
			}
			qtyStr, _ := t.FormatQuantity(symbol, rawQty)
			parsedQty, _ := strconv.ParseFloat(qtyStr, 64)
			if parsedQty <= 0 {
				continue
			}

			targetPriceDeltaPct := plan[i].TargetROE / leverage
			var tpPrice float64
			if side == "LONG" {
				tpPrice = entryPrice * (1 + targetPriceDeltaPct)
			} else {
				tpPrice = entryPrice * (1 - targetPriceDeltaPct)
			}

			out = append(out, TPOrder{
				BatchIndex: i,
				Qty:        parsedQty,
				Price:      tpPrice,
			})
			remainingToAllocate -= parsedQty
			if remainingToAllocate <= 0 {
				break
			}
		}
		return out
	}

	effectiveBatches := batches
	ordersToPlace = buildOrders(effectiveBatches, filledBatches)

	// Precision fallback for tiny positions (common on BTC small-USDT notional):
	// compact TP ladder 0.5% + 1.0% (50% + 50%)
	if len(ordersToPlace) < 2 {
		compactOrders := buildOrders(compactBatches, filledBatches)
		if len(compactOrders) > 0 {
			effectiveBatches = compactBatches
			ordersToPlace = compactOrders
			logger.Infof("🛡️ %s %s precision-limited position -> use compact TP ladder (2 levels)", symbol, side)
		}
	}

	expectedTPs := len(ordersToPlace)
	if expectedTPs == 0 && amount > 0 {
		// Dust fallback: keep one TP so the position is not left without profit exits.
		batchIdx := filledBatches
		if batchIdx >= len(effectiveBatches) {
			batchIdx = len(effectiveBatches) - 1
		}
		if batchIdx < 0 {
			batchIdx = 0
		}
		targetPriceDeltaPct := effectiveBatches[batchIdx].TargetROE / leverage
		var tpPrice float64
		if side == "LONG" {
			tpPrice = entryPrice * (1 + targetPriceDeltaPct)
		} else {
			tpPrice = entryPrice * (1 - targetPriceDeltaPct)
		}
		ordersToPlace = append(ordersToPlace, TPOrder{
			BatchIndex: batchIdx,
			Qty:        amount,
			Price:      tpPrice,
		})
		expectedTPs = 1
	}

	useDualSL := progress == nil || !progress.MultiSLDisabled
	targetSLROEs := stageStopLossROEs(filledBatches, useDualSL, slCfg)
	targetSLPrices := make([]float64, 0, len(targetSLROEs))
	for _, roe := range targetSLROEs {
		targetSLPrices = append(targetSLPrices, stopLossPriceFromROE(entryPrice, leverage, side, roe))
	}
	slSetMatched := stopLossSetMatches(slPrices, targetSLPrices)

	// After first successful takeover, only stage-adjust SL; keep TP ladder untouched.
	if progress != nil && progress.EnforcedOnce {
		if slSetMatched {
			return nil
		}
		if err := w.rebuildStageStopLosses(t, symbol, side, amount, entryPrice, leverage, filledBatches, progress, slCfg); err != nil {
			return fmt.Errorf("failed to rebuild staged stop-loss set: %w", err)
		}
		return nil
	}

	if slCount == len(targetSLPrices) && tpCount == expectedTPs && slSetMatched {
		if progress != nil {
			progress.EnforcedOnce = true
		}
		return nil
	}

	logger.Infof("🛡️ Risk Mismatch for %s %s (SL: %d, TP: %d, Expected: %d) -> Enforcing ROE Protocol (Lev: %.1fx)", symbol, side, slCount, tpCount, expectedTPs, leverage)

	if err := t.CancelStopOrders(symbol); err != nil {
		return fmt.Errorf("failed to cancel old orders: %w", err)
	}

	for idx, slPrice := range targetSLPrices {
		if err := t.SetStopLoss(symbol, side, amount, slPrice); err != nil {
			logger.Errorf("🛡️ Failed to set Stop Loss #%d for %s: %v", idx+1, symbol, err)
		} else {
			priceDeltaPct := math.Abs(slPrice-entryPrice) / entryPrice
			roePct := priceDeltaPct * leverage * 100
			sign := "-"
			if (side == "LONG" && slPrice > entryPrice) || (side == "SHORT" && slPrice < entryPrice) {
				sign = "+"
			}
			logger.Infof("🛡️ Set Stage SL #%d for %s @ %.4f (%s%.2f%% ROE)", idx+1, symbol, slPrice, sign, roePct)
		}
	}

	for _, order := range ordersToPlace {
		if err := t.SetPartialTakeProfit(symbol, side, order.Qty, order.Price); err != nil {
			logger.Warnf("🛡️ Failed to set TP Batch %d (Qty: %.4f) for %s: %v", order.BatchIndex+1, order.Qty, symbol, err)
		} else {
			priceDeltaPct := math.Abs(order.Price-entryPrice) / entryPrice
			roePct := priceDeltaPct * leverage * 100
			logger.Infof("  ✓ Set TP Batch %d: +%.1f%% ROE @ %.4f (Qty: %.4f)", order.BatchIndex+1, roePct, order.Price, order.Qty)
		}
	}

	if progress != nil {
		progress.EnforcedOnce = true
	}

	return nil
}

func isRiskOrderType(orderType string) bool {
	return orderType == "STOP_MARKET" ||
		orderType == "TAKE_PROFIT_MARKET" ||
		orderType == "STOP" ||
		orderType == "TAKE_PROFIT"
}

func (w *RiskWatcher) updatePositionProgress(traderID, symbol, side string, amount, entryPrice float64, plan []tpBatch) *positionProgress {
	key := traderID + "|" + symbol + "|" + side
	p, ok := w.progress[key]
	if !ok {
		p = &positionProgress{
			InitialAmount:   amount,
			LastAmount:      amount,
			LastEntryPrice:  entryPrice,
			FilledBatches:   0,
			EnforcedOnce:    false,
			MultiSLDisabled: false,
		}
		w.progress[key] = p
		return p
	}

	// Position cycle changed (closed then re-opened quickly): reset ladder baseline.
	// This avoids inheriting old filled-stage state when symbol stays active between scans.
	if p.LastEntryPrice > 0 {
		priceChangeRatio := math.Abs(entryPrice-p.LastEntryPrice) / p.LastEntryPrice
		if priceChangeRatio > 0.0005 { // 0.05%
			logger.Infof("🛡️ %s %s entry changed (%.6f -> %.6f), reset ladder baseline",
				symbol, side, p.LastEntryPrice, entryPrice)
			p.InitialAmount = amount
			p.LastAmount = amount
			p.LastEntryPrice = entryPrice
			p.FilledBatches = 0
			p.EnforcedOnce = false
			p.MultiSLDisabled = false
			return p
		}
	}

	// Position increase/new add-on: reset baseline to avoid stale ladder stage.
	if amount > p.LastAmount*1.02 {
		logger.Infof("🛡️ %s %s position increased (%.4f -> %.4f), reset ladder baseline", symbol, side, p.LastAmount, amount)
		p.InitialAmount = amount
		p.FilledBatches = 0
		p.EnforcedOnce = false
		p.MultiSLDisabled = false
	}
	if p.InitialAmount <= 0 {
		p.InitialAmount = amount
	}

	filled := inferFilledBatches(amount, p.InitialAmount, plan)
	if filled > p.FilledBatches {
		ratio := 0.0
		if p.InitialAmount > 0 {
			ratio = amount / p.InitialAmount
		}
		logger.Infof("🛡️ %s %s ladder advanced: initial=%.4f current=%.4f ratio=%.4f filled=%d->%d",
			symbol, side, p.InitialAmount, amount, ratio, p.FilledBatches, filled)
		p.FilledBatches = filled
	}
	maxFilled := len(plan)
	if maxFilled <= 0 {
		maxFilled = 4
	}
	if p.FilledBatches > maxFilled {
		p.FilledBatches = maxFilled
	}
	p.LastAmount = amount
	p.LastEntryPrice = entryPrice
	return p
}

func (w *RiskWatcher) clearSymbolProgress(traderID, symbol string) {
	delete(w.progress, traderID+"|"+symbol+"|LONG")
	delete(w.progress, traderID+"|"+symbol+"|SHORT")
}

func inferFilledBatches(currentAmount, initialAmount float64, plan []tpBatch) int {
	if initialAmount <= 0 || currentAmount <= 0 {
		return 0
	}
	ratio := currentAmount / initialAmount
	eps := 0.015
	filled := 0
	remain := 1.0
	for _, batch := range plan {
		remain -= batch.PosPct
		if remain < 0 {
			remain = 0
		}
		if ratio <= remain+eps {
			filled++
		}
	}
	return filled
}

func stageStopLossROEs(filledBatches int, dual bool, cfg stagedStopLoss) []float64 {
	if filledBatches < 0 {
		filledBatches = 0
	}

	if !dual {
		if filledBatches >= 2 {
			return []float64{cfg.AfterTP2ROE}
		}
		if filledBatches >= 1 {
			return []float64{cfg.AfterTP1ROE}
		}
		return []float64{cfg.InitialROE}
	}

	// Dual-SL strategy:
	// stage0: [initial]
	// stage1: [initial, after tp1]
	// stage2+: [after tp1, after tp2]
	if filledBatches >= 2 {
		return []float64{cfg.AfterTP1ROE, cfg.AfterTP2ROE}
	}
	if filledBatches >= 1 {
		return []float64{cfg.InitialROE, cfg.AfterTP1ROE}
	}
	return []float64{cfg.InitialROE}
}

func stopLossPriceFromROE(entryPrice, leverage float64, side string, targetROE float64) float64 {
	if leverage < 1 {
		leverage = 1
	}
	priceDeltaPct := math.Abs(targetROE) / leverage

	if targetROE < 0 {
		if side == "LONG" {
			return entryPrice * (1 - priceDeltaPct)
		}
		return entryPrice * (1 + priceDeltaPct)
	}

	if side == "LONG" {
		return entryPrice * (1 + priceDeltaPct)
	}
	return entryPrice * (1 - priceDeltaPct)
}

func priceNearlyEqual(a, b float64) bool {
	diff := math.Abs(a - b)
	tol := math.Max(0.02, math.Abs(b)*0.00002) // max(0.02, 0.002%)
	return diff <= tol
}

func stopLossSetMatches(existingPrices, targetPrices []float64) bool {
	if len(existingPrices) != len(targetPrices) {
		return false
	}

	used := make([]bool, len(targetPrices))
	for _, existing := range existingPrices {
		matched := false
		for i, target := range targetPrices {
			if used[i] {
				continue
			}
			if priceNearlyEqual(existing, target) {
				used[i] = true
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func (w *RiskWatcher) rebuildStageStopLosses(
	t *binance.FuturesTrader,
	symbol, side string,
	amount, entryPrice, leverage float64,
	filledBatches int,
	progress *positionProgress,
	cfg stagedStopLoss,
) error {
	useDualSL := progress == nil || !progress.MultiSLDisabled
	targetROEs := stageStopLossROEs(filledBatches, useDualSL, cfg)

	targetPrices := make([]float64, 0, len(targetROEs))
	for _, roe := range targetROEs {
		targetPrices = append(targetPrices, stopLossPriceFromROE(entryPrice, leverage, side, roe))
	}

	if err := t.CancelStopLossOrdersForSide(symbol, side); err != nil {
		return fmt.Errorf("failed to cancel old staged stop-loss orders: %w", err)
	}

	success := 0
	for idx, price := range targetPrices {
		if err := t.SetStopLoss(symbol, side, amount, price); err != nil {
			logger.Warnf("🛡️ Failed to set staged SL #%d for %s %s @ %.4f: %v", idx+1, symbol, side, price, err)
			continue
		}
		success++
	}

	if success == len(targetPrices) {
		return nil
	}

	// Fallback: if dual-SL placement fails, downgrade to single dynamic SL mode.
	if useDualSL && progress != nil && !progress.MultiSLDisabled {
		progress.MultiSLDisabled = true
		logger.Warnf("🛡️ Dual-SL placement not fully accepted for %s %s, fallback to single-SL mode", symbol, side)
		return w.rebuildStageStopLosses(t, symbol, side, amount, entryPrice, leverage, filledBatches, progress, cfg)
	}

	if success == 0 {
		return fmt.Errorf("failed to place any staged stop-loss orders")
	}

	return nil
}

func normalizeExitConfig(riskCfg store.RiskControlConfig) ([]tpBatch, []tpBatch, stagedStopLoss) {
	defaultPlan := []tpBatch{
		{TargetROE: 0.005, PosPct: 0.20},
		{TargetROE: 0.010, PosPct: 0.20},
		{TargetROE: 0.020, PosPct: 0.30},
		{TargetROE: 0.030, PosPct: 0.20},
	}

	plan := make([]tpBatch, 0, 4)
	for _, level := range riskCfg.TakeProfitLevels {
		if len(plan) >= 4 {
			break
		}
		targetROE := level.TargetROEPct / 100.0
		posPct := level.ClosePct / 100.0
		if targetROE <= 0 || posPct <= 0 {
			continue
		}
		plan = append(plan, tpBatch{TargetROE: targetROE, PosPct: posPct})
	}
	if len(plan) == 0 {
		plan = append(plan, defaultPlan...)
	}

	sum := 0.0
	for _, b := range plan {
		sum += b.PosPct
	}
	if sum > 1 {
		scale := 1 / sum
		for i := range plan {
			plan[i].PosPct *= scale
		}
	}

	compact := []tpBatch{
		{TargetROE: 0.005, PosPct: 0.50},
		{TargetROE: 0.010, PosPct: 0.50},
	}
	if len(plan) >= 2 {
		compact[0].TargetROE = plan[0].TargetROE
		compact[1].TargetROE = plan[1].TargetROE
	} else if len(plan) == 1 {
		compact = []tpBatch{
			{TargetROE: plan[0].TargetROE, PosPct: 1.00},
		}
	}

	slCfg := stagedStopLoss{
		InitialROE:  -0.016,
		AfterTP1ROE: -0.0055,
		AfterTP2ROE: 0.001,
	}
	if riskCfg.StagedStopLoss != (store.StagedStopLossConfig{}) {
		slCfg.InitialROE = riskCfg.StagedStopLoss.InitialROEPct / 100.0
		slCfg.AfterTP1ROE = riskCfg.StagedStopLoss.AfterTP1ROEPct / 100.0
		slCfg.AfterTP2ROE = riskCfg.StagedStopLoss.AfterTP2ROEPct / 100.0
	}

	return plan, compact, slCfg
}

func remainingRatioAfterFilled(plan []tpBatch, filled int) float64 {
	if filled <= 0 {
		return 1.0
	}
	if filled > len(plan) {
		filled = len(plan)
	}
	sumFilled := 0.0
	for i := 0; i < filled; i++ {
		sumFilled += plan[i].PosPct
	}
	remain := 1.0 - sumFilled
	if remain < 0 {
		return 0
	}
	return remain
}
