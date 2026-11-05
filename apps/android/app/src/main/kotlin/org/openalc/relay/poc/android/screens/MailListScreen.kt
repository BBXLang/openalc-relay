package org.openalc.relay.poc.android.screens

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import org.openalc.relay.poc.android.R
import org.openalc.relay.poc.android.components.StateBlock
import org.openalc.relay.poc.android.components.StateBlockKind

/**
 * Formal mail list. Formal sync is not connected yet, so this renders the honest empty state via
 * StateBlock — never fabricated mail. The LazyListState is hoisted in RelayAppState so returning
 * from a detail page or search restores the original list position.
 */
@Composable
fun MailListScreen(listState: LazyListState, modifier: Modifier = Modifier) {
    Box(modifier.fillMaxSize()) {
        LazyColumn(state = listState, modifier = Modifier.fillMaxSize()) {
            item(key = "mail-empty-state") {
                Box(Modifier.fillParentMaxSize()) {
                    StateBlock(
                        kind = StateBlockKind.EMPTY,
                        message = stringResource(R.string.formal_sync_not_connected),
                        detail = stringResource(R.string.mail_list_empty_detail),
                    )
                }
            }
        }
    }
}
