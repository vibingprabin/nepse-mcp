import 'package:flutter/material.dart';
import '../data/mock_data.dart';
import '../models/portfolio.dart';
import '../widgets/stock_holding_card.dart';

class PortfolioScreen extends StatefulWidget {
  const PortfolioScreen({super.key});

  @override
  State<PortfolioScreen> createState() => _PortfolioScreenState();
}

class _PortfolioScreenState extends State<PortfolioScreen> {
  late Portfolio _portfolio;
  String _sortBy = 'VALUE';

  @override
  void initState() {
    super.initState();
    _portfolio = MockData.getPortfolio();
  }

  @override
  Widget build(BuildContext context) {
    final unrealizedPL = _portfolio.unrealizedPL;
    final unrealizedPercent = (unrealizedPL / _portfolio.totalInvestment) * 100;
    final realizedPL = -20000.0; // Mock data
    final realizedPercent = -1.2;

    return Scaffold(
      backgroundColor: const Color(0xFFF5F7F9),
      appBar: AppBar(
        backgroundColor: Colors.white,
        elevation: 0,
        title: const Text(
          'Portfolio',
          style: TextStyle(
            fontSize: 20,
            fontWeight: FontWeight.w600,
            color: Color(0xFF1A1A1A),
          ),
        ),
        actions: [
          IconButton(
            icon: const Icon(
              Icons.add_circle_outline,
              color: Color(0xFF1A1A1A),
            ),
            onPressed: () {},
          ),
        ],
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          _buildTotalValueCard(),
          const SizedBox(height: 16),
          Row(
            children: [
              Expanded(
                child: _buildPLCard(
                  'Unrealised P&L',
                  unrealizedPL,
                  unrealizedPercent,
                  true,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _buildPLCard(
                  'Realised P&L',
                  realizedPL,
                  realizedPercent,
                  false,
                ),
              ),
            ],
          ),
          const SizedBox(height: 24),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              const Text(
                'Your Holdings',
                style: TextStyle(
                  fontSize: 17,
                  fontWeight: FontWeight.w600,
                  color: Color(0xFF1A1A1A),
                ),
              ),
              GestureDetector(
                onTap: () {
                  // Toggle sort
                },
                child: Row(
                  children: [
                    Text(
                      _sortBy,
                      style: const TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w500,
                        color: Color(0xFF1FA89C),
                      ),
                    ),
                    const Icon(
                      Icons.keyboard_arrow_down,
                      size: 20,
                      color: Color(0xFF1FA89C),
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: 16),
          ..._portfolio.holdings.map(
            (holding) => StockHoldingCard(holding: holding),
          ),
        ],
      ),
    );
  }

  Widget _buildTotalValueCard() {
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Total Value',
            style: TextStyle(
              fontSize: 13,
              color: Color(0xFF6B7280),
              fontWeight: FontWeight.w500,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            _portfolio.formattedCurrentValue,
            style: const TextStyle(
              fontSize: 28,
              fontWeight: FontWeight.w700,
              color: Color(0xFF1A1A1A),
              letterSpacing: -0.5,
            ),
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              Icon(
                _portfolio.isProfit ? Icons.arrow_upward : Icons.arrow_downward,
                size: 16,
                color: _portfolio.isProfit
                    ? const Color(0xFF00C853)
                    : const Color(0xFFFF3B30),
              ),
              const SizedBox(width: 4),
              Text(
                _portfolio.formattedProfitLoss,
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  color: _portfolio.isProfit
                      ? const Color(0xFF00C853)
                      : const Color(0xFFFF3B30),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildPLCard(
    String title,
    double value,
    double percent,
    bool isPositive,
  ) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: const TextStyle(
              fontSize: 11,
              color: Color(0xFF6B7280),
              fontWeight: FontWeight.w500,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            '${isPositive ? '+' : '-'} ${value.abs().toStringAsFixed(0)}',
            style: TextStyle(
              fontSize: 20,
              fontWeight: FontWeight.w700,
              color: isPositive
                  ? const Color(0xFF00C853)
                  : const Color(0xFFFF3B30),
            ),
          ),
          const SizedBox(height: 4),
          Text(
            '${percent.toStringAsFixed(1)}%',
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w500,
              color: isPositive
                  ? const Color(0xFF00C853)
                  : const Color(0xFFFF3B30),
            ),
          ),
        ],
      ),
    );
  }
}
