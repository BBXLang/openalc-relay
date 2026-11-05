package org.openalc.relay.poc.android.components

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.openalc.relay.poc.android.R
import org.openalc.relay.poc.android.theme.LocalRelayPalette

/**
 * Unified rendering for list/page states: loading, empty, offline, failed.
 * Formal screens must show honest reasons; they never render fabricated session data.
 */
enum class StateBlockKind { LOADING, EMPTY, OFFLINE, FAILED }

@Composable
fun StateBlock(
    kind: StateBlockKind,
    message: String,
    modifier: Modifier = Modifier,
    detail: String? = null,
    onRetry: (() -> Unit)? = null,
) {
    val palette = LocalRelayPalette.current
    Box(modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        Column(
            modifier = Modifier.padding(28.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            if (kind == StateBlockKind.LOADING) {
                CircularProgressIndicator(color = palette.ink)
            }
            Text(
                message,
                fontSize = 14.sp,
                lineHeight = 20.sp,
                color = palette.ink,
                textAlign = TextAlign.Center,
            )
            if (detail != null) {
                Text(
                    detail,
                    fontSize = 12.sp,
                    lineHeight = 17.sp,
                    color = palette.muted,
                    textAlign = TextAlign.Center,
                )
            }
            if (onRetry != null && kind != StateBlockKind.LOADING) {
                Button(
                    onClick = onRetry,
                    colors = ButtonDefaults.buttonColors(
                        containerColor = palette.ink,
                        contentColor = palette.canvas,
                    ),
                ) {
                    Text(stringResource(R.string.state_block_retry), fontSize = 13.sp)
                }
            }
        }
    }
}
