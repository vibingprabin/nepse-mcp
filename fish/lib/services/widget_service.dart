import 'dart:convert';
import 'package:home_widget/home_widget.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../models/market_event.dart';
import '../models/portfolio.dart';

/// Service to manage Android home screen widget data and updates
class WidgetService {
  static const String _widgetGroupId = 'nepse_fisher_widgets';

  // Widget data keys
  static const String _keyNepseIndex = 'nepse_index';
  static const String _keyIndexChange = 'index_change';
  static const String _keyIndexChangePercent = 'index_change_percent';
  static const String _keyMarketStatus = 'market_status';
  static const String _keyLatestEvents = 'latest_events';
  static const String _keyPortfolioValue = 'portfolio_value';
  static const String _keyPortfolioChange = 'portfolio_change';
  static const String _keyLastUpdate = 'last_update';

  /// Initialize the widget service
  static Future<void> initialize() async {
    await HomeWidget.setAppGroupId(_widgetGroupId);

    // Register callback for widget interactions
    HomeWidget.registerInteractivityCallback(_handleWidgetAction);
  }

  /// Update all widget data
  static Future<void> updateWidgetData({
    required double nepseIndex,
    required double indexChange,
    required double indexChangePercent,
    required bool isMarketOpen,
    required List<MarketEvent> latestEvents,
    required Portfolio portfolio,
  }) async {
    // Save basic market data
    await HomeWidget.saveWidgetData<double>(_keyNepseIndex, nepseIndex);
    await HomeWidget.saveWidgetData<double>(_keyIndexChange, indexChange);
    await HomeWidget.saveWidgetData<double>(
      _keyIndexChangePercent,
      indexChangePercent,
    );
    await HomeWidget.saveWidgetData<String>(
      _keyMarketStatus,
      isMarketOpen ? 'OPEN' : 'CLOSED',
    );

    // Save portfolio data
    await HomeWidget.saveWidgetData<double>(
      _keyPortfolioValue,
      portfolio.currentValue,
    );
    await HomeWidget.saveWidgetData<double>(
      _keyPortfolioChange,
      portfolio.totalProfitLossPercent,
    );

    // Save latest events as JSON
    final eventsJson = latestEvents
        .take(5)
        .map(
          (event) => {
            'symbol': event.symbol,
            'company': event.companyName,
            'price': event.price,
            'changePercent': event.changePercent,
            'eventType': event.eventType,
            'description': event.description,
            'timestamp': event.timestamp.toIso8601String(),
          },
        )
        .toList();

    await HomeWidget.saveWidgetData<String>(
      _keyLatestEvents,
      jsonEncode(eventsJson),
    );

    // Save last update timestamp
    await HomeWidget.saveWidgetData<String>(
      _keyLastUpdate,
      DateTime.now().toIso8601String(),
    );

    // Trigger widget update
    await updateWidget();
  }

  /// Trigger widget UI update
  static Future<void> updateWidget() async {
    try {
      await HomeWidget.updateWidget(
        androidName: 'NepseWidgetProvider',
        iOSName: 'NepseWidget',
      );
    } catch (e) {
      print('Error updating widget: $e');
    }
  }

  /// Handle widget action callbacks
  static Future<void> _handleWidgetAction(Uri? uri) async {
    if (uri == null) return;

    final action = uri.host;
    final params = uri.queryParameters;

    print('Widget action: $action, params: $params');

    // Handle different widget actions
    switch (action) {
      case 'refresh':
        // Trigger data refresh
        await _refreshWidgetData();
        break;
      case 'open_stock':
        final symbol = params['symbol'];
        if (symbol != null) {
          // Navigate to stock detail
          await _navigateToStock(symbol);
        }
        break;
      case 'open_portfolio':
        // Navigate to portfolio screen
        await _navigateToPortfolio();
        break;
      case 'open_events':
        // Navigate to events screen
        await _navigateToEvents();
        break;
      default:
        print('Unknown widget action: $action');
    }
  }

  /// Refresh widget data from app
  static Future<void> _refreshWidgetData() async {
    // This will be called when user taps refresh button
    // The actual data refresh should be handled by the app
    print('Widget refresh requested');
  }

  /// Navigate to stock detail screen
  static Future<void> _navigateToStock(String symbol) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('pending_navigation', 'stock:$symbol');
  }

  /// Navigate to portfolio screen
  static Future<void> _navigateToPortfolio() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('pending_navigation', 'portfolio');
  }

  /// Navigate to events screen
  static Future<void> _navigateToEvents() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('pending_navigation', 'events');
  }

  /// Get pending navigation and clear it
  static Future<String?> getPendingNavigation() async {
    final prefs = await SharedPreferences.getInstance();
    final navigation = prefs.getString('pending_navigation');
    if (navigation != null) {
      await prefs.remove('pending_navigation');
    }
    return navigation;
  }

  /// Check if market is currently open (simplified - you may want more complex logic)
  static bool isMarketOpen() {
    final now = DateTime.now();
    final dayOfWeek = now.weekday;
    final hour = now.hour;

    // Market open Sunday-Thursday, 11 AM - 3 PM (Nepal time)
    if (dayOfWeek >= 1 && dayOfWeek <= 4) {
      if (hour >= 11 && hour < 15) {
        return true;
      }
    }

    return false;
  }
}
