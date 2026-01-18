class StockHolding {
  final String symbol;
  final String companyName;
  final int quantity;
  final double avgPrice;
  final double ltp; // Last Traded Price

  StockHolding({
    required this.symbol,
    required this.companyName,
    required this.quantity,
    required this.avgPrice,
    required this.ltp,
  });

  double get totalValue => quantity * ltp;
  double get totalCost => quantity * avgPrice;
  double get profitLoss => totalValue - totalCost;
  double get profitLossPercent => ((ltp - avgPrice) / avgPrice) * 100;

  bool get isProfit => profitLoss >= 0;

  String get formattedLTP => ltp.toStringAsFixed(2);
  String get formattedAvgPrice => avgPrice.toStringAsFixed(2);
  String get formattedProfitLoss {
    final sign = profitLoss >= 0 ? '+' : '';
    return '$sign${profitLossPercent.toStringAsFixed(1)}%';
  }
}
