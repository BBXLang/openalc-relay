package org.openalc.relay.poc.android.screens

import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import org.openalc.relay.poc.android.R

/**
 * Mail detail page. Formal sync is not connected, so the message content, attachments and status
 * trail cannot be shown; the page renders the honest state instead of fabricated content.
 */
@Composable
fun MailDetailScreen(id: String, onBack: () -> Unit, modifier: Modifier = Modifier) {
    DetailUnavailableScreen(
        title = stringResource(R.string.detail_mail_title),
        id = id,
        onBack = onBack,
        modifier = modifier,
    )
}
