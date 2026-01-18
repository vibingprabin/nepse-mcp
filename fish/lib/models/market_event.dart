class MarketEvent {
  final String symbol;
  final String companyName;
  final double price;
  final double changePercent;
  final String eventType;
  final String description;
  final DateTime timestamp;
  final List<String> tags;

  MarketEvent({
    required this.symbol,
    required this.companyName,
    required this.price,
    required this.changePercent,
    required this.eventType,
    required this.description,
    required this.timestamp,
    this.tags = const [],
  });

  bool get isPositive => changePercent >= 0;

  String get formattedPrice => 'Rs. ${price.toStringAsFixed(2)}';

  String get formattedChange {
    final sign = changePercent >= 0 ? '+' : '';
    return '$sign${changePercent.toStringAsFixed(1)}%';
  }
}
