package org.openalc.relay.poc.android.screens

import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import org.openalc.relay.poc.android.R

/**
 * Device detail page (paired device, SIMs, permissions, last observation, revoke state).
 * Formal pairing is not connected, so the page renders the honest state instead of device data.
 */
@Composable
fun DeviceDetailScreen(id: String, onBack: () -> Unit, modifier: Modifier = Modifier) {
    DetailUnavailableScreen(
        title = stringResource(R.string.detail_device_title),
        id = id,
        onBack = onBack,
        modifier = modifier,
    )
}
