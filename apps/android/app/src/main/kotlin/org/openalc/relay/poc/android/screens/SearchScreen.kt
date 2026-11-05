package org.openalc.relay.poc.android.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.openalc.relay.poc.android.R
import org.openalc.relay.poc.android.components.DetailTopBar
import org.openalc.relay.poc.android.components.SearchGlyph
import org.openalc.relay.poc.android.components.StateBlock
import org.openalc.relay.poc.android.components.StateBlockKind
import org.openalc.relay.poc.android.navigation.RelayChannel
import org.openalc.relay.poc.android.theme.LocalRelayPalette

/**
 * Channel-scoped search page (plan section 2.1/2.3). Results never mix channels; clearing the
 * query or going back restores the original list, whose scroll state lives in RelayAppState.
 * Formal search is not connected yet, so the result area shows the honest state.
 */
@Composable
fun SearchScreen(channel: RelayChannel, onBack: () -> Unit, modifier: Modifier = Modifier) {
    val palette = LocalRelayPalette.current
    var query by remember { mutableStateOf("") }
    val isSms = channel == RelayChannel.SMS

    Column(
        modifier
            .fillMaxSize()
            .statusBarsPadding()
            .navigationBarsPadding(),
    ) {
        DetailTopBar(
            title = stringResource(if (isSms) R.string.search_in_sms else R.string.search_in_mail),
            onBack = onBack,
        )
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 20.dp)
                .background(palette.surface, RoundedCornerShape(50))
                .padding(horizontal = 14.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(Modifier.size(20.dp)) { SearchGlyph(palette.muted) }
            BasicTextField(
                value = query,
                onValueChange = { query = it },
                modifier = Modifier.weight(1f).padding(horizontal = 10.dp),
                singleLine = true,
                textStyle = TextStyle(color = palette.ink, fontSize = 14.sp),
                cursorBrush = SolidColor(palette.ink),
                decorationBox = { inner ->
                    Box {
                        if (query.isEmpty()) {
                            Text(
                                stringResource(R.string.search_field_hint),
                                color = palette.muted,
                                fontSize = 14.sp,
                            )
                        }
                        inner()
                    }
                },
            )
            if (query.isNotEmpty()) {
                Text(
                    stringResource(R.string.search_clear),
                    modifier = Modifier.clickable { query = "" },
                    fontSize = 12.sp,
                    color = palette.muted,
                )
            }
        }
        Spacer(Modifier.height(8.dp))
        Text(
            stringResource(if (isSms) R.string.search_scope_sms else R.string.search_scope_mail),
            modifier = Modifier.padding(horizontal = 28.dp, vertical = 6.dp),
            fontSize = 11.sp,
            color = palette.muted,
        )
        Box(Modifier.weight(1f)) {
            StateBlock(
                kind = StateBlockKind.EMPTY,
                message = stringResource(R.string.search_unavailable),
                detail = stringResource(
                    if (isSms) R.string.search_unavailable_detail_sms else R.string.search_unavailable_detail_mail,
                ),
            )
        }
    }
}
