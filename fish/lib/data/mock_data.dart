import '../models/market_event.dart';
import '../models/stock_holding.dart';
import '../models/portfolio.dart';

class MockData {
  static List<MarketEvent> getMarketEvents() {
    return [
      MarketEvent(
        symbol: 'NABIL',
        companyName: 'Nabil Bank Ltd.',
        price: 490.80,
        changePercent: 2.4,
        eventType: 'Price Breakout',
        description:
            'Significant accumulation observed near the 52-week support zone.',
        timestamp: DateTime.now().subtract(const Duration(minutes: 23)),
        tags: ['accumulation', 'support-zone'],
      ),
      MarketEvent(
        symbol: 'NICA',
        companyName: 'NIC Asia Bank',
        price: 720.00,
        changePercent: 0.8,
        eventType: 'Volume Expansion',
        description:
            'Institutional volume detected exceeding daily average by 40%.',
        timestamp: DateTime.now().subtract(const Duration(minutes: 45)),
        tags: [],
      ),
      MarketEvent(
        symbol: 'HDHPC',
        companyName: 'Himal Dolakha Hydropower',
        price: 145.00,
        changePercent: -1.1,
        eventType: 'Support Test',
        description: 'Testing key support level at 145.',
        timestamp: DateTime.now().subtract(
          const Duration(hours: 1, minutes: 20),
        ),
        tags: [],
      ),
      MarketEvent(
        symbol: 'SHIVM',
        companyName: 'Shivam Cements Ltd',
        price: 512.00,
        changePercent: 0.0,
        eventType: 'Range Bound',
        description: 'Consolidating in tight range, awaiting breakout.',
        timestamp: DateTime.now().subtract(
          const Duration(hours: 2, minutes: 30),
        ),
        tags: [],
      ),
    ];
  }

  static List<StockHolding> getStockHoldings() {
    return [
      StockHolding(
        symbol: 'NABIL',
        companyName: 'NABIL BANK LTD.',
        quantity: 500,
        avgPrice: 420.50,
        ltp: 482.00,
      ),
      StockHolding(
        symbol: 'NICA',
        companyName: 'NIC ASIA BANK',
        quantity: 100,
        avgPrice: 450.00,
        ltp: 740.00,
      ),
      StockHolding(
        symbol: 'HIDCL',
        companyName: 'HYDROELECTRICITY INV.',
        quantity: 1000,
        avgPrice: 200.00,
        ltp: 185.00,
      ),
    ];
  }

  static Portfolio getPortfolio() {
    return Portfolio(holdings: getStockHoldings(), totalInvestment: 2450000);
  }

  static List<Map<String, dynamic>> getHistoryLog() {
    return [
      {
        'time': '14:30',
        'symbol': 'NICA',
        'company': 'Nic Asia Bank Ltd.',
        'change': 12.00,
        'percent': 1.6,
        'date': 'TODAY 24 OCT',
      },
      {
        'time': '12:15',
        'symbol': 'HDHPC',
        'company': 'Himal Dolakha Hydropower',
        'change': -4.00,
        'percent': 0.9,
        'date': 'TODAY 24 OCT',
      },
      {
        'time': '11:45',
        'symbol': 'NABIL',
        'company': 'Nabil Bank Limited',
        'change': 2.50,
        'percent': 0.5,
        'date': 'TODAY 24 OCT',
      },
      {
        'time': '11:05',
        'symbol': 'AKPL',
        'company': 'Arun Kabeli Power Ltd.',
        'change': -1.10,
        'percent': 0.2,
        'date': 'TODAY 24 OCT',
      },
      {
        'time': '10:15',
        'symbol': 'NRIC',
        'company': 'Nepali Reinsurance Co.',
        'change': 51.00,
        'percent': 4.1,
        'date': 'TODAY 24 OCT',
      },
      {
        'time': '14:55',
        'symbol': 'PCBL',
        'company': 'Prime Commercial Bank',
        'change': 1.00,
        'percent': 0.1,
        'date': 'YESTERDAY 23 OCT',
      },
      {
        'time': '13:20',
        'symbol': 'SHIVM',
        'company': 'Shivam Cements Ltd',
        'change': -15.00,
        'percent': 2.3,
        'date': 'YESTERDAY 23 OCT',
      },
      {
        'time': '11:10',
        'symbol': 'HIDCL',
        'company': 'Hydroelectricity Inv.',
        'change': 0.00,
        'percent': 0.0,
        'date': 'YESTERDAY 23 OCT',
      },
    ];
  }
}
