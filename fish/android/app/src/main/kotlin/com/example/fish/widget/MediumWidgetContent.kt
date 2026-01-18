package com.example.fish.widget

import androidx.compose.runtime.Composable
import androidx.glance.text.Text

@Composable
fun MediumWidgetContent(
    nepseIndex: Double,
    indexChange: Double,
    indexChangePercent: Double,
    marketStatus: String,
    latestEventsJson: String
) {
    Text("Medium Widget")
}
