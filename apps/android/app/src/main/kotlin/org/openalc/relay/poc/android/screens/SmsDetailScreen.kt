package org.openalc.relay.poc.android.screens

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import org.openalc.relay.poc.android.R
import org.openalc.relay.poc.android.components.DetailTopBar
import org.openalc.relay.poc.android.components.StateBlock
import org.openalc.relay.poc.android.components.StateBlockKind

/**
 * SMS detail page. Formal sync is not connected, so the full message content cannot be shown;
 * the page renders the honest state with the requested record key instead of fabricated content.
 */
@Composable
fun SmsDetailScreen(id: String, onBack: () -> Unit, modifier: Modifier = Modifier) {
    DetailUnavailableScreen(
        title = stringResource(R.string.detail_sms_title),
        id = id,
        onBack = onBack,
        modifier = modifier,
    )
}

/** Shared honest body for detail pages whose formal data is not connected yet. */
@Composable
internal fun DetailUnavailableScreen(
    title: String,
    id: String,
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier
            .fillMaxSize()
            .statusBarsPadding()
            .navigationBarsPadding(),
    ) {
        DetailTopBar(title = title, onBack = onBack)
        StateBlock(
            kind = StateBlockKind.EMPTY,
            message = stringResource(R.string.detail_unavailable),
            detail = stringResource(R.string.detail_id_label, id),
        )
    }
}
