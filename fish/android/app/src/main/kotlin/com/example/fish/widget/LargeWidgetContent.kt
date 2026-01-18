package com.example.fish.widget

import androidx.compose.runtime.Composable
import androidx.glance.text.Text

@Composable
fun LargeWidgetContent(
    nepseIndex: Double,
    indexChange: Double,
    indexChangePercent: Double,
    marketStatus: String,
    latestEventsJson: String,
    portfolioValue: Double,
    portfolioChange: Double
) {
    Text("Large Widget")
}
