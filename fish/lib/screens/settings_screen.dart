import 'package:flutter/material.dart';

class SettingsScreen extends StatefulWidget {
  const SettingsScreen({super.key});

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  bool _pushNotifications = true;
  bool _priceAlerts = true;
  bool _signalUpdates = false;
  bool _autoRefresh = true;
  bool _dataSaverMode = false;
  bool _includeDPCharge = true;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFFF5F7F9),
      appBar: AppBar(
        backgroundColor: Colors.white,
        elevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back, color: Color(0xFF1A1A1A)),
          onPressed: () => Navigator.pop(context),
        ),
        title: const Text(
          'Settings',
          style: TextStyle(
            fontSize: 17,
            fontWeight: FontWeight.w600,
            color: Color(0xFF1A1A1A),
          ),
        ),
      ),
      body: ListView(
        children: [
          _buildSectionHeader('PREFERENCES'),
          _buildSettingItem(
            'Push Notifications',
            _pushNotifications,
            (value) => setState(() => _pushNotifications = value),
          ),
          _buildSettingItem(
            'Price Alerts',
            _priceAlerts,
            (value) => setState(() => _priceAlerts = value),
          ),
          _buildSettingItem(
            'Signal Updates',
            _signalUpdates,
            (value) => setState(() => _signalUpdates = value),
          ),
          const SizedBox(height: 16),
          _buildSectionHeader('MARKET DATA'),
          _buildSettingItem(
            'Auto-Refresh',
            _autoRefresh,
            (value) => setState(() => _autoRefresh = value),
          ),
          _buildNavigationItem('Refresh Interval', '10s'),
          _buildSettingItem(
            'Data Saver Mode',
            _dataSaverMode,
            (value) => setState(() => _dataSaverMode = value),
            subtitle: 'Updates only on WiFi',
          ),
          const SizedBox(height: 16),
          _buildSectionHeader('PORTFOLIO SETTINGS'),
          _buildNavigationItem('Default Broker', 'Broker 58'),
          _buildNavigationItem('Transaction Fees', 'Standard SEBON'),
          _buildSettingItem(
            'Include DP Charge',
            _includeDPCharge,
            (value) => setState(() => _includeDPCharge = value),
          ),
          const SizedBox(height: 32),
          const Padding(
            padding: EdgeInsets.all(16),
            child: Column(
              children: [
                Text(
                  'NEPSE Fisher v1.0.2',
                  style: TextStyle(
                    fontSize: 11,
                    color: Color(0xFF9CA3AF),
                    fontWeight: FontWeight.w500,
                  ),
                ),
                SizedBox(height: 4),
                Text(
                  'Market data provided by NEPSE',
                  style: TextStyle(fontSize: 11, color: Color(0xFF9CA3AF)),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSectionHeader(String title) {
    return Container(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
      child: Text(
        title,
        style: const TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: Color(0xFF6B7280),
          letterSpacing: 0.5,
        ),
      ),
    );
  }

  Widget _buildSettingItem(
    String title,
    bool value,
    Function(bool) onChanged, {
    String? subtitle,
  }) {
    return Container(
      color: Colors.white,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: const TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w400,
                    color: Color(0xFF1A1A1A),
                  ),
                ),
                if (subtitle != null) ...[
                  const SizedBox(height: 2),
                  Text(
                    subtitle,
                    style: const TextStyle(
                      fontSize: 11,
                      color: Color(0xFF9CA3AF),
                    ),
                  ),
                ],
              ],
            ),
          ),
          Switch(
            value: value,
            onChanged: onChanged,
            thumbColor: WidgetStateProperty.resolveWith<Color>(
              (states) => states.contains(WidgetState.selected)
                  ? const Color(0xFF1FA89C)
                  : Colors.grey,
            ),
            trackColor: WidgetStateProperty.resolveWith<Color>(
              (states) => states.contains(WidgetState.selected)
                  ? const Color.fromRGBO(31, 168, 156, 0.5)
                  : const Color.fromRGBO(158, 158, 158, 0.3),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildNavigationItem(String title, String value) {
    return Container(
      color: Colors.white,
      child: ListTile(
        contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
        title: Text(
          title,
          style: const TextStyle(
            fontSize: 15,
            fontWeight: FontWeight.w400,
            color: Color(0xFF1A1A1A),
          ),
        ),
        trailing: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              value,
              style: const TextStyle(fontSize: 13, color: Color(0xFF9CA3AF)),
            ),
            const SizedBox(width: 8),
            const Icon(Icons.chevron_right, color: Color(0xFF9CA3AF), size: 20),
          ],
        ),
        onTap: () {},
      ),
    );
  }
}
