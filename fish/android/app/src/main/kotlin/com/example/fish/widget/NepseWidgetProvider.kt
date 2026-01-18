package com.example.fish.widget

import android.content.Context
import androidx.glance.appwidget.GlanceAppWidget
import androidx.glance.appwidget.GlanceAppWidgetReceiver
import androidx.glance.appwidget.provideContent
import androidx.glance.GlanceId
import androidx.glance.appwidget.SizeMode
import androidx.glance.state.PreferencesGlanceStateDefinition
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.doublePreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.glance.LocalSize
import androidx.compose.ui.unit.dp
import androidx.glance.currentState

/**
 * Main widget provider for NEPSE Fisher widgets
 * Supports three sizes: Small, Medium, and Large
 */
class NepseWidgetProvider : GlanceAppWidgetReceiver() {
    override val glanceAppWidget: GlanceAppWidget = NepseWidget()
}

class NepseWidget : GlanceAppWidget() {
    
    // Support different widget sizes
    override val sizeMode = SizeMode.Exact
    
    override val stateDefinition = PreferencesGlanceStateDefinition
    
    override suspend fun provideGlance(context: Context, id: GlanceId) {
        provideContent {
            val prefs = currentState<Preferences>()
            
            // Read widget data from preferences
            val nepseIndex = prefs[WidgetDataKeys.NEPSE_INDEX] ?: 0.0
            val indexChange = prefs[WidgetDataKeys.INDEX_CHANGE] ?: 0.0
            val indexChangePercent = prefs[WidgetDataKeys.INDEX_CHANGE_PERCENT] ?: 0.0
            val marketStatus = prefs[WidgetDataKeys.MARKET_STATUS] ?: "CLOSED"
            val latestEvents = prefs[WidgetDataKeys.LATEST_EVENTS] ?: "[]"
            val portfolioValue = prefs[WidgetDataKeys.PORTFOLIO_VALUE] ?: 0.0
            val portfolioChange = prefs[WidgetDataKeys.PORTFOLIO_CHANGE] ?: 0.0
            
            // Determine which widget to show based on available size
            val size = LocalSize.current
            
            when {
                // Small widget: width < 250dp or height < 200dp
                size.width < 250.dp || size.height < 200.dp -> {
                    SmallWidgetContent(
                        nepseIndex = nepseIndex,
                        indexChange = indexChange,
                        indexChangePercent = indexChangePercent,
                        marketStatus = marketStatus
                    )
                }
                // Large widget: width > 300dp and height > 300dp
                size.width > 300.dp && size.height > 300.dp -> {
                    LargeWidgetContent(
                        nepseIndex = nepseIndex,
                        indexChange = indexChange,
                        indexChangePercent = indexChangePercent,
                        marketStatus = marketStatus,
                        latestEventsJson = latestEvents,
                        portfolioValue = portfolioValue,
                        portfolioChange = portfolioChange
                    )
                }
                // Medium widget: everything else
                else -> {
                    MediumWidgetContent(
                        nepseIndex = nepseIndex,
                        indexChange = indexChange,
                        indexChangePercent = indexChangePercent,
                        marketStatus = marketStatus,
                        latestEventsJson = latestEvents
                    )
                }
            }
        }
    }
}

// Preference keys for widget data
object WidgetDataKeys {
    val NEPSE_INDEX = doublePreferencesKey("nepse_index")
    val INDEX_CHANGE = doublePreferencesKey("index_change")
    val INDEX_CHANGE_PERCENT = doublePreferencesKey("index_change_percent")
    val MARKET_STATUS = stringPreferencesKey("market_status")
    val LATEST_EVENTS = stringPreferencesKey("latest_events")
    val PORTFOLIO_VALUE = doublePreferencesKey("portfolio_value")
    val PORTFOLIO_CHANGE = doublePreferencesKey("portfolio_change")
    val LAST_UPDATE = stringPreferencesKey("last_update")
}
