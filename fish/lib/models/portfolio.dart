import 'stock_holding.dart';

class Portfolio {
  final List<StockHolding> holdings;
  final double totalInvestment;

  Portfolio({required this.holdings, required this.totalInvestment});

  double get currentValue {
    return holdings.fold(0.0, (sum, holding) => sum + holding.totalValue);
  }

  double get totalProfitLoss => currentValue - totalInvestment;

  double get totalProfitLossPercent {
    if (totalInvestment == 0) return 0;
    return (totalProfitLoss / totalInvestment) * 100;
  }

  double get unrealizedPL {
    return holdings.fold(0.0, (sum, holding) => sum + holding.profitLoss);
  }

  bool get isProfit => totalProfitLoss >= 0;

  String get formattedCurrentValue {
    return 'NPR ${_formatNumber(currentValue)}';
  }

  String get formattedProfitLoss {
    final sign = totalProfitLoss >= 0 ? '+' : '';
    return 'NPR $sign${_formatNumber(totalProfitLoss.abs())} (${totalProfitLossPercent.toStringAsFixed(1)}%)';
  }

  String _formatNumber(double number) {
    if (number >= 10000000) {
      return '${(number / 10000000).toStringAsFixed(2)}Cr';
    } else if (number >= 100000) {
      return '${(number / 100000).toStringAsFixed(2)}L';
    } else if (number >= 1000) {
      return '${(number / 1000).toStringAsFixed(0)}K';
    }
    return number.toStringAsFixed(2);
  }
}
